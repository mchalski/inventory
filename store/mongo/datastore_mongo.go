// Copyright 2020 Northern.tech AS
//
//    Licensed under the Apache License, Version 2.0 (the "License");
//    you may not use this file except in compliance with the License.
//    You may obtain a copy of the License at
//
//        http://www.apache.org/licenses/LICENSE-2.0
//
//    Unless required by applicable law or agreed to in writing, software
//    distributed under the License is distributed on an "AS IS" BASIS,
//    WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//    See the License for the specific language governing permissions and
//    limitations under the License.

package mongo

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	mopts "go.mongodb.org/mongo-driver/mongo/options"

	"github.com/mendersoftware/go-lib-micro/log"
	mstore "github.com/mendersoftware/go-lib-micro/store"
	"github.com/pkg/errors"

	"github.com/mendersoftware/inventory/model"
	"github.com/mendersoftware/inventory/store"
)

const (
	DbVersion = "1.0.0"

	DbName        = "inventory"
	DbDevicesColl = "devices"

	DbDevId              = "_id"
	DbDevAttributes      = "attributes"
	DbDevGroup           = "group"
	DbDevAttributesDesc  = "description"
	DbDevAttributesValue = "value"
	DbDevAttributesScope = "scope"
	DbDevAttributesName  = "name"
	DbDevAttributesGroup = DbDevAttributes + "." +
		model.AttrScopeSystem + "-" + model.AttrNameGroup
	DbDevAttributesGroupValue = DbDevAttributesGroup + "." +
		DbDevAttributesValue

	DbScopeInventory = "inventory"
)

var (
	//with offcial mongodb supported driver we keep client
	clientGlobal *mongo.Client

	// once ensures client is created only once
	once sync.Once

	ErrNotFound = errors.New("mongo: no documents in result")
)

type DataStoreMongoConfig struct {
	// connection string
	ConnectionString string

	// SSL support
	SSL           bool
	SSLSkipVerify bool

	// Overwrites credentials provided in connection string if provided
	Username string
	Password string
}

type DataStoreMongo struct {
	client      *mongo.Client
	automigrate bool
}

func NewDataStoreMongoWithSession(client *mongo.Client) store.DataStore {
	return &DataStoreMongo{client: client}
}

//config.ConnectionString must contain a valid
func NewDataStoreMongo(config DataStoreMongoConfig) (store.DataStore, error) {
	//init master session
	var err error
	once.Do(func() {
		if !strings.Contains(config.ConnectionString, "://") {
			config.ConnectionString = "mongodb://" + config.ConnectionString
		}
		clientOptions := mopts.Client().ApplyURI(config.ConnectionString)

		if config.Username != "" {
			clientOptions.SetAuth(mopts.Credential{
				Username: config.Username,
				Password: config.Password,
			})
		}

		if config.SSL {
			tlsConfig := &tls.Config{}
			tlsConfig.InsecureSkipVerify = config.SSLSkipVerify
			clientOptions.SetTLSConfig(tlsConfig)
		}

		ctx := context.Background()
		l := log.FromContext(ctx)
		clientGlobal, err = mongo.Connect(ctx, clientOptions)
		if err != nil {
			l.Errorf("mongo: error connecting to mongo '%s'", err.Error())
			return
		}
		if clientGlobal == nil {
			l.Errorf("mongo: client is nil. wow.")
			return
		}
		// from: https://www.mongodb.com/blog/post/mongodb-go-driver-tutorial
		/*
			It is best practice to keep a client that is connected to MongoDB around so that the application can make use of connection pooling - you don't want to open and close a connection for each query. However, if your application no longer requires a connection, the connection can be closed with client.Disconnect() like so:
		*/
		err = clientGlobal.Ping(ctx, nil)
		if err != nil {
			clientGlobal = nil
			l.Errorf("mongo: error pinging mongo '%s'", err.Error())
			return
		}
		if clientGlobal == nil {
			l.Errorf("mongo: global instance of client is nil.")
			return
		}
	})

	if clientGlobal == nil {
		return nil, errors.New("failed to open mongo-driver session")
	}
	db := &DataStoreMongo{client: clientGlobal}

	return db, nil
}

type internalDeviceResult struct {
	Devices    []model.Device `bson:"results"`
	TotalCount int            `bson:"totalCount"`
}

func (db *DataStoreMongo) GetDevices(ctx context.Context, q store.ListQuery) ([]model.Device, int, error) {
	c := db.client.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)

	queryFilters := make([]bson.M, 0)
	for _, filter := range q.Filters {
		op := mongoOperator(filter.Operator)
		name := fmt.Sprintf("%s-%s", filter.AttrScope, filter.AttrName)
		field := fmt.Sprintf("%s.%s.%s", DbDevAttributes, name, DbDevAttributesValue)
		switch filter.Operator {
		default:
			if filter.ValueFloat != nil {
				queryFilters = append(queryFilters, bson.M{"$or": []bson.M{
					{field: bson.M{op: filter.Value}},
					{field: bson.M{op: filter.ValueFloat}},
				}})
			} else {
				queryFilters = append(queryFilters, bson.M{field: bson.M{op: filter.Value}})
			}
		}
	}
	findQuery := bson.M{}
	if len(queryFilters) > 0 {
		findQuery["$and"] = queryFilters
	}
	groupFilter := bson.M{}
	if q.GroupName != "" {
		groupFilter = bson.M{DbDevAttributesGroupValue: q.GroupName}
	}
	groupExistenceFilter := bson.M{}
	if q.HasGroup != nil {
		groupExistenceFilter = bson.M{
			DbDevAttributesGroup: bson.M{
				"$exists": *q.HasGroup,
			},
		}
	}
	filter := bson.M{
		"$match": bson.M{
			"$and": []bson.M{
				groupFilter,
				groupExistenceFilter,
				findQuery,
			},
		},
	}

	// since the sorting step will have to be executable we have to use a noop here instead of just
	// an empty query object, as unsorted queries would fail otherwise
	sortQuery := bson.M{"$skip": 0}
	if q.Sort != nil {
		name := fmt.Sprintf("%s-%s", q.Sort.AttrScope, q.Sort.AttrName)
		sortField := fmt.Sprintf("%s.%s.%s", DbDevAttributes, name, DbDevAttributesValue)
		sortFieldQuery := bson.M{}
		sortFieldQuery[sortField] = 1
		if !q.Sort.Ascending {
			sortFieldQuery[sortField] = -1
		}
		sortQuery = bson.M{"$sort": sortFieldQuery}
	}
	limitQuery := bson.M{"$skip": 0}
	// exchange the limit query only if limit is set, as limits need to be positive in an aggregation pipeline
	if q.Limit > 0 {
		limitQuery = bson.M{"$limit": q.Limit}
	}
	combinedQuery := bson.M{
		"$facet": bson.M{
			"results": []bson.M{
				sortQuery,
				{"$skip": q.Skip},
				limitQuery,
			},
			"totalCount": []bson.M{
				{"$count": "count"},
			},
		},
	}
	resultMap := bson.M{
		"$project": bson.M{
			"results": 1,
			"totalCount": bson.M{
				"$ifNull": []interface{}{
					bson.M{
						"$arrayElemAt": []interface{}{"$totalCount.count", 0},
					},
					0,
				},
			},
		},
	}

	cursor, err := c.Aggregate(ctx, []bson.M{
		filter,
		combinedQuery,
		resultMap,
	})
	defer cursor.Close(ctx)

	if !cursor.Next(ctx) {
		return nil, 0, nil
	}
	res := internalDeviceResult{}
	if err = cursor.Decode(&res); err != nil {
		return nil, -1, errors.Wrap(err, "failed to fetch device list")
	}
	return res.Devices, res.TotalCount, nil
}

func (db *DataStoreMongo) GetDevice(
	ctx context.Context,
	id model.DeviceID,
) (*model.Device, error) {
	var res model.Device
	c := db.client.
		Database(mstore.DbFromContext(ctx, DbName)).
		Collection(DbDevicesColl)
	l := log.FromContext(ctx)

	if id == model.NilDeviceID {
		return nil, nil
	}
	if err := c.FindOne(ctx, bson.M{DbDevId: id}).Decode(&res); err != nil {
		switch err {
		case mongo.ErrNoDocuments:
			return nil, nil
		default:
			l.Errorf("GetDevice: %v", err)
			return nil, errors.Wrap(err, "failed to fetch device")
		}
	}
	return &res, nil
}

// AddDevice inserts a new device, initializing the inventory data.
func (db *DataStoreMongo) AddDevice(ctx context.Context, dev *model.Device) error {
	if dev.Group != "" {
		dev.Attributes = append(dev.Attributes, model.DeviceAttribute{
			Scope: model.AttrScopeSystem,
			Name:  model.AttrNameGroup,
			Value: dev.Group,
		})
	}
	err := db.UpsertAttributes(ctx, dev.ID, dev.Attributes)
	if err != nil {
		return errors.Wrap(err, "failed to store device")
	}
	return nil
}

// UpsertAttributes makes an upsert on the device's attributes.
func (db *DataStoreMongo) UpsertAttributes(ctx context.Context, id model.DeviceID, attrs model.DeviceAttributes) error {
	const systemScope = DbDevAttributes + "." + model.AttrScopeSystem
	const updatedField = systemScope + "-" + model.AttrNameUpdated
	const createdField = systemScope + "-" + model.AttrNameCreated

	c := db.client.
		Database(mstore.DbFromContext(ctx, DbName)).
		Collection(DbDevicesColl)

	filter := bson.M{"_id": id}
	update, err := makeAttrUpsert(attrs)
	if err != nil {
		return err
	}
	now := time.Now()
	update[updatedField] = model.DeviceAttribute{
		Scope: model.AttrScopeSystem,
		Name:  model.AttrNameUpdated,
		Value: now,
	}
	update = bson.M{
		"$set": update,
		"$setOnInsert": bson.M{
			createdField: model.DeviceAttribute{
				Scope: model.AttrScopeSystem,
				Name:  model.AttrNameCreated,
				Value: now,
			},
		},
	}
	_, err = c.UpdateOne(ctx, filter, update, mopts.Update().SetUpsert(true))
	if err != nil {
		return err
	}
	return nil
}

// makeAttrField is a convenience function for composing attribute field names.
func makeAttrField(attrName, attrScope string, subFields ...string) string {
	field := fmt.Sprintf("%s.%s-%s", DbDevAttributes, attrScope, attrName)
	if len(subFields) > 0 {
		field = strings.Join(
			append([]string{field}, subFields...), ".",
		)
	}
	return field
}

// makeAttrUpsert creates a new upsert document for the given attributes.
func makeAttrUpsert(attrs model.DeviceAttributes) (bson.M, error) {
	var fieldName string
	upsert := make(bson.M)

	for i := range attrs {
		if attrs[i].Name == "" {
			return nil, store.ErrNoAttrName
		}
		if attrs[i].Scope == "" {
			// Default to inventory scope
			attrs[i].Scope = model.AttrScopeInventory
		}

		fieldName = makeAttrField(
			attrs[i].Name,
			attrs[i].Scope,
			DbDevAttributesScope,
		)
		upsert[fieldName] = attrs[i].Scope

		fieldName = makeAttrField(
			attrs[i].Name,
			attrs[i].Scope,
			DbDevAttributesName,
		)
		upsert[fieldName] = attrs[i].Name

		if attrs[i].Value != nil {
			fieldName = makeAttrField(
				attrs[i].Name,
				attrs[i].Scope,
				DbDevAttributesValue,
			)
			upsert[fieldName] = attrs[i].Value
		}

		if attrs[i].Description != nil {
			fieldName = makeAttrField(
				attrs[i].Name,
				attrs[i].Scope,
				DbDevAttributesDesc,
			)
			upsert[fieldName] = attrs[i].Description

		}
	}
	return upsert, nil
}

func mongoOperator(co store.ComparisonOperator) string {
	switch co {
	case store.Eq:
		return "$eq"
	}
	return ""
}

func (db *DataStoreMongo) UnsetDeviceGroup(ctx context.Context, id model.DeviceID, groupName model.GroupName) error {
	c := db.client.
		Database(mstore.DbFromContext(ctx, DbName)).
		Collection(DbDevicesColl)

	filter := bson.M{
		"_id":                     id,
		DbDevAttributesGroupValue: groupName,
	}
	update := bson.M{
		"$unset": bson.M{
			DbDevAttributesGroup: 1,
		},
	}

	res, err := c.UpdateMany(ctx, filter, update)
	if err != nil {
		return err
	}
	if res.ModifiedCount <= 0 {
		return store.ErrDevNotFound
	}
	return nil
}

func (db *DataStoreMongo) UpdateDeviceGroup(ctx context.Context, devId model.DeviceID, newGroup model.GroupName) error {
	c := db.client.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)

	filter := bson.M{
		"_id": devId,
	}
	update := bson.M{
		"$set": bson.M{
			DbDevAttributesGroup: model.DeviceAttribute{
				Scope: model.AttrScopeSystem,
				Name:  DbDevGroup,
				Value: newGroup,
			},
		},
	}

	res, err := c.UpdateOne(ctx, filter, update)
	if err != nil {
		return err
	}

	if res.ModifiedCount > 0 {
		return nil
	} else {
		return store.ErrDevNotFound
	} // to check the update count
}

func (db *DataStoreMongo) UpdateDevicesGroup(
	ctx context.Context,
	devIDs []model.DeviceID,
	group model.GroupName,
) (int64, int64, error) {
	database := db.client.Database(mstore.DbFromContext(ctx, DbName))
	collDevs := database.Collection(DbDevicesColl)

	filter := bson.M{
		DbDevId: bson.M{"$in": devIDs},
	}
	update := bson.M{
		"$set": bson.M{
			DbDevAttributesGroup: model.DeviceAttribute{
				Scope: model.AttrScopeSystem,
				Name:  DbDevGroup,
				Value: group,
			},
		},
	}
	res, err := collDevs.UpdateMany(ctx, filter, update)
	if err != nil {
		return -1, -1, err
	}
	return res.MatchedCount, res.ModifiedCount, nil
}

func (db *DataStoreMongo) UnsetDevicesGroup(
	ctx context.Context,
	deviceIDs []model.DeviceID,
	group model.GroupName,
) (int64, error) {
	database := db.client.Database(mstore.DbFromContext(ctx, DbName))
	collDevs := database.Collection(DbDevicesColl)

	filter := bson.D{
		{Key: DbDevId, Value: bson.M{"$in": deviceIDs}},
		{Key: DbDevAttributesGroupValue, Value: group},
	}
	update := bson.M{
		"$unset": bson.M{
			DbDevAttributesGroup: "",
		},
	}
	res, err := collDevs.UpdateMany(ctx, filter, update)
	if err != nil {
		return -1, err
	}
	return res.ModifiedCount, nil

}

func (db *DataStoreMongo) ListGroups(ctx context.Context) ([]model.GroupName, error) {
	c := db.client.
		Database(mstore.DbFromContext(ctx, DbName)).
		Collection(DbDevicesColl)

	filter := bson.M{DbDevAttributesGroupValue: bson.M{"$exists": true}}
	results, err := c.Distinct(
		ctx, DbDevAttributesGroupValue, filter,
	)
	if err != nil {
		return nil, err
	}

	groups := make([]model.GroupName, len(results))
	for i, d := range results {
		groups[i] = model.GroupName(d.(string))
	}
	return groups, nil
}

func (db *DataStoreMongo) GetDevicesByGroup(ctx context.Context, group model.GroupName, skip, limit int) ([]model.DeviceID, int, error) {
	c := db.client.
		Database(mstore.DbFromContext(ctx, DbName)).
		Collection(DbDevicesColl)

	filter := bson.M{DbDevAttributesGroupValue: group}
	result := c.FindOne(ctx, filter)
	if result == nil {
		return nil, -1, store.ErrGroupNotFound
	}

	var dev model.Device
	err := result.Decode(&dev)
	if err != nil {
		return nil, -1, store.ErrGroupNotFound
	}

	hasGroup := group != ""
	devices, totalDevices, e := db.GetDevices(ctx,
		store.ListQuery{
			Skip:      skip,
			Limit:     limit,
			Filters:   nil,
			Sort:      nil,
			HasGroup:  &hasGroup,
			GroupName: string(group)})
	if e != nil {
		return nil, -1, errors.Wrap(e, "failed to get device list for group")
	}

	resIds := make([]model.DeviceID, len(devices))
	for i, d := range devices {
		resIds[i] = d.ID
	}
	return resIds, totalDevices, nil
}

func (db *DataStoreMongo) GetDeviceGroup(ctx context.Context, id model.DeviceID) (model.GroupName, error) {
	dev, err := db.GetDevice(ctx, id)
	if err != nil || dev == nil {
		return "", store.ErrDevNotFound
	}

	return dev.Group, nil
}

func (db *DataStoreMongo) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	c := db.client.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)

	filter := bson.M{DbDevId: id}
	result, err := c.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if result.DeletedCount < 1 {
		return store.ErrDevNotFound
	} // to check the delete count

	return nil
}

func (db *DataStoreMongo) GetAllAttributeNames(ctx context.Context) ([]string, error) {
	c := db.client.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)

	project := bson.M{
		"$project": bson.M{
			"arrayofkeyvalue": bson.M{
				"$objectToArray": "$$ROOT.attributes",
			},
		},
	}

	unwind := bson.M{
		"$unwind": "$arrayofkeyvalue",
	}

	group := bson.M{
		"$group": bson.M{
			"_id": nil,
			"allkeys": bson.M{
				"$addToSet": "$arrayofkeyvalue.v.name",
			},
		},
	}

	l := log.FromContext(ctx)
	cursor, err := c.Aggregate(ctx, []bson.M{
		project,
		unwind,
		group,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	cursor.Next(ctx)
	elem := &bson.D{}
	err = cursor.Decode(elem)
	if err != nil {
		if err != io.EOF {
			return nil, errors.Wrap(err, "failed to get attributes")
		} else {
			return make([]string, 0), nil
		}
	}
	m := elem.Map()
	results := m["allkeys"].(primitive.A)
	attributeNames := make([]string, len(results))
	for i, d := range results {
		attributeNames[i] = d.(string)
		l.Debugf("GetAllAttributeNames got: '%v'", d)
	}

	return attributeNames, nil
}

func (db *DataStoreMongo) SearchDevices(ctx context.Context, searchParams model.SearchParams) ([]model.Device, int, error) {
	c := db.client.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)

	queryFilters := make([]bson.M, 0)
	for _, filter := range searchParams.Filters {
		op := filter.Type
		name := fmt.Sprintf("%s-%s", filter.Scope, filter.Attribute)
		field := fmt.Sprintf("%s.%s.%s", DbDevAttributes, name, DbDevAttributesValue)
		queryFilters = append(queryFilters, bson.M{field: bson.M{op: filter.Value}})
	}

	// FIXME: remove after migrating ids to attributes
	if len(searchParams.DeviceIDs) > 0 {
		queryFilters = append(queryFilters, bson.M{"_id": bson.M{"$in": searchParams.DeviceIDs}})
	}

	findQuery := bson.M{}
	if len(queryFilters) > 0 {
		findQuery["$and"] = queryFilters
	}

	findOptions := mopts.Find()
	findOptions.SetSkip(int64((searchParams.Page - 1) * searchParams.PerPage))
	findOptions.SetLimit(int64(searchParams.PerPage))

	if len(searchParams.Sort) > 0 {
		sortField := bson.M{}
		for _, sortQ := range searchParams.Sort {
			name := fmt.Sprintf("%s-%s", sortQ.Scope, sortQ.Attribute)
			field := fmt.Sprintf("%s.%s.%s", DbDevAttributes, name, DbDevAttributesValue)
			sortField[field] = 1
			if sortQ.Order == "desc" {
				sortField[field] = -1
			}
		}
		findOptions.SetSort(sortField)
	}

	cursor, err := c.Find(ctx, findQuery, findOptions)
	if err != nil {
		return nil, -1, errors.Wrap(err, "failed to search devices")
	}
	defer cursor.Close(ctx)

	devices := []model.Device{}

	if err = cursor.All(ctx, &devices); err != nil {
		return nil, -1, errors.Wrap(err, "failed to search devices")
	}

	count, err := c.CountDocuments(ctx, findQuery)

	return devices, int(count), nil
}

func indexAttr(s *mongo.Client, ctx context.Context, attr string) error {
	l := log.FromContext(ctx)
	c := s.Database(mstore.DbFromContext(ctx, DbName)).Collection(DbDevicesColl)
	indexField := fmt.Sprintf("attributes.%s.values", attr)

	indexView := c.Indexes()
	_, err := indexView.CreateOne(ctx, mongo.IndexModel{Keys: bson.M{indexField: 1}, Options: nil})

	if err != nil {
		if isTooManyIndexes(err) {
			l.Warnf("failed to index attr %s in db %s: too many indexes", attr, mstore.DbFromContext(ctx, DbName))
		} else {
			return errors.Wrapf(err, "failed to index attr %s in db %s", attr, mstore.DbFromContext(ctx, DbName))
		}
	}

	return nil
}

func isTooManyIndexes(e error) bool {
	return strings.HasPrefix(e.Error(), "add index fails, too many indexes for inventory.devices")
}
