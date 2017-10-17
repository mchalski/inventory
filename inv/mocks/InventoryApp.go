// Copyright 2017 Northern.tech AS
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
package mocks

import context "context"

import mock "github.com/stretchr/testify/mock"
import model "github.com/mendersoftware/inventory/model"
import store "github.com/mendersoftware/inventory/store"

// InventoryApp is an autogenerated mock type for the InventoryApp type
type InventoryApp struct {
	mock.Mock
}

// AddDevice provides a mock function with given fields: ctx, d
func (_m *InventoryApp) AddDevice(ctx context.Context, d *model.Device) error {
	ret := _m.Called(ctx, d)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, *model.Device) error); ok {
		r0 = rf(ctx, d)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// CreateTenant provides a mock function with given fields: ctx, tenant
func (_m *InventoryApp) CreateTenant(ctx context.Context, tenant model.NewTenant) error {
	ret := _m.Called(ctx, tenant)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, model.NewTenant) error); ok {
		r0 = rf(ctx, tenant)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// DeleteDevice provides a mock function with given fields: ctx, id
func (_m *InventoryApp) DeleteDevice(ctx context.Context, id model.DeviceID) error {
	ret := _m.Called(ctx, id)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID) error); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// GetDevice provides a mock function with given fields: ctx, id
func (_m *InventoryApp) GetDevice(ctx context.Context, id model.DeviceID) (*model.Device, error) {
	ret := _m.Called(ctx, id)

	var r0 *model.Device
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID) *model.Device); ok {
		r0 = rf(ctx, id)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*model.Device)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, model.DeviceID) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// GetDeviceGroup provides a mock function with given fields: ctx, id
func (_m *InventoryApp) GetDeviceGroup(ctx context.Context, id model.DeviceID) (model.GroupName, error) {
	ret := _m.Called(ctx, id)

	var r0 model.GroupName
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID) model.GroupName); ok {
		r0 = rf(ctx, id)
	} else {
		r0 = ret.Get(0).(model.GroupName)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, model.DeviceID) error); ok {
		r1 = rf(ctx, id)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListDevices provides a mock function with given fields: ctx, skip, limit, filters, sort, hasGroup
func (_m *InventoryApp) ListDevices(ctx context.Context, skip int, limit int, filters []store.Filter, sort *store.Sort, hasGroup *bool) ([]model.Device, error) {
	ret := _m.Called(ctx, skip, limit, filters, sort, hasGroup)

	var r0 []model.Device
	if rf, ok := ret.Get(0).(func(context.Context, int, int, []store.Filter, *store.Sort, *bool) []model.Device); ok {
		r0 = rf(ctx, skip, limit, filters, sort, hasGroup)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.Device)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, int, int, []store.Filter, *store.Sort, *bool) error); ok {
		r1 = rf(ctx, skip, limit, filters, sort, hasGroup)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListDevicesByGroup provides a mock function with given fields: ctx, group, skip, limit
func (_m *InventoryApp) ListDevicesByGroup(ctx context.Context, group model.GroupName, skip int, limit int) ([]model.DeviceID, error) {
	ret := _m.Called(ctx, group, skip, limit)

	var r0 []model.DeviceID
	if rf, ok := ret.Get(0).(func(context.Context, model.GroupName, int, int) []model.DeviceID); ok {
		r0 = rf(ctx, group, skip, limit)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.DeviceID)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context, model.GroupName, int, int) error); ok {
		r1 = rf(ctx, group, skip, limit)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// ListGroups provides a mock function with given fields: ctx
func (_m *InventoryApp) ListGroups(ctx context.Context) ([]model.GroupName, error) {
	ret := _m.Called(ctx)

	var r0 []model.GroupName
	if rf, ok := ret.Get(0).(func(context.Context) []model.GroupName); ok {
		r0 = rf(ctx)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]model.GroupName)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(context.Context) error); ok {
		r1 = rf(ctx)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}

// UnsetDeviceGroup provides a mock function with given fields: ctx, id, groupName
func (_m *InventoryApp) UnsetDeviceGroup(ctx context.Context, id model.DeviceID, groupName model.GroupName) error {
	ret := _m.Called(ctx, id, groupName)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID, model.GroupName) error); ok {
		r0 = rf(ctx, id, groupName)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpdateDeviceGroup provides a mock function with given fields: ctx, id, group
func (_m *InventoryApp) UpdateDeviceGroup(ctx context.Context, id model.DeviceID, group model.GroupName) error {
	ret := _m.Called(ctx, id, group)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID, model.GroupName) error); ok {
		r0 = rf(ctx, id, group)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// UpsertAttributes provides a mock function with given fields: ctx, id, attrs
func (_m *InventoryApp) UpsertAttributes(ctx context.Context, id model.DeviceID, attrs model.DeviceAttributes) error {
	ret := _m.Called(ctx, id, attrs)

	var r0 error
	if rf, ok := ret.Get(0).(func(context.Context, model.DeviceID, model.DeviceAttributes) error); ok {
		r0 = rf(ctx, id, attrs)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
