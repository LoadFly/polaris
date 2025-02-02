/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package service_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/pkg/errors"
	apifault "github.com/polarismesh/specification/source/go/api/v1/fault_tolerance"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
	apiservice "github.com/polarismesh/specification/source/go/api/v1/service_manage"
	"github.com/stretchr/testify/assert"

	"github.com/polarismesh/polaris/common/utils"
	"github.com/polarismesh/polaris/service"
)

func TestServer_CreateCircuitBreakerJson(t *testing.T) {
	rule := &apifault.CircuitBreaker{}
	rule.Id = &wrappers.StringValue{Value: "12345678"}
	rule.Version = &wrappers.StringValue{Value: "1.0.0"}
	rule.Name = &wrappers.StringValue{Value: "testCbRule"}
	rule.Namespace = &wrappers.StringValue{Value: "Test"}
	rule.Service = &wrappers.StringValue{Value: "TestService1"}
	rule.ServiceNamespace = &wrappers.StringValue{Value: "Test"}
	rule.Inbounds = []*apifault.CbRule{
		{
			Sources: []*apifault.SourceMatcher{
				{
					Service:   &wrappers.StringValue{Value: "*"},
					Namespace: &wrappers.StringValue{Value: "*"},
					Labels: map[string]*apimodel.MatchString{
						"user": {
							Type:  0,
							Value: &wrappers.StringValue{Value: "vip"},
						},
					},
				},
			},
			Destinations: []*apifault.DestinationSet{
				{
					Method: &apimodel.MatchString{
						Type:  0,
						Value: &wrappers.StringValue{Value: "/info"},
					},
					Resource: apifault.DestinationSet_INSTANCE,
					Type:     apifault.DestinationSet_LOCAL,
					Scope:    apifault.DestinationSet_CURRENT,
					Policy: &apifault.CbPolicy{
						ErrorRate: &apifault.CbPolicy_ErrRateConfig{
							Enable:                 &wrappers.BoolValue{Value: true},
							RequestVolumeThreshold: &wrappers.UInt32Value{Value: 10},
							ErrorRateToOpen:        &wrappers.UInt32Value{Value: 50},
						},
						Consecutive: &apifault.CbPolicy_ConsecutiveErrConfig{
							Enable:                 &wrappers.BoolValue{Value: true},
							ConsecutiveErrorToOpen: &wrappers.UInt32Value{Value: 10},
						},
						SlowRate: &apifault.CbPolicy_SlowRateConfig{
							Enable:         &wrappers.BoolValue{Value: true},
							MaxRt:          &duration.Duration{Seconds: 1},
							SlowRateToOpen: &wrappers.UInt32Value{Value: 80},
						},
					},
					Recover: &apifault.RecoverConfig{
						SleepWindow: &duration.Duration{
							Seconds: 1,
						},
						OutlierDetectWhen: apifault.RecoverConfig_ON_RECOVER,
					},
				},
			},
		},
	}
	rule.Outbounds = []*apifault.CbRule{
		{
			Sources: []*apifault.SourceMatcher{
				{
					Labels: map[string]*apimodel.MatchString{
						"callerName": {
							Type:  0,
							Value: &wrappers.StringValue{Value: "xyz"},
						},
					},
				},
			},
			Destinations: []*apifault.DestinationSet{
				{
					Namespace: &wrappers.StringValue{Value: "Test"},
					Service:   &wrappers.StringValue{Value: "TestService1"},
					Method: &apimodel.MatchString{
						Type:  0,
						Value: &wrappers.StringValue{Value: "/info"},
					},
					Resource: apifault.DestinationSet_INSTANCE,
					Type:     apifault.DestinationSet_LOCAL,
					Scope:    apifault.DestinationSet_CURRENT,
					Policy: &apifault.CbPolicy{
						ErrorRate: &apifault.CbPolicy_ErrRateConfig{
							Enable:                 &wrappers.BoolValue{Value: true},
							RequestVolumeThreshold: &wrappers.UInt32Value{Value: 10},
							ErrorRateToOpen:        &wrappers.UInt32Value{Value: 50},
						},
						Consecutive: &apifault.CbPolicy_ConsecutiveErrConfig{
							Enable:                 &wrappers.BoolValue{Value: true},
							ConsecutiveErrorToOpen: &wrappers.UInt32Value{Value: 10},
						},
						SlowRate: &apifault.CbPolicy_SlowRateConfig{
							Enable:         &wrappers.BoolValue{Value: true},
							MaxRt:          &duration.Duration{Seconds: 1},
							SlowRateToOpen: &wrappers.UInt32Value{Value: 80},
						},
					},
					Recover: &apifault.RecoverConfig{
						SleepWindow: &duration.Duration{
							Seconds: 1,
						},
						OutlierDetectWhen: apifault.RecoverConfig_ON_RECOVER,
					},
				},
			},
		},
	}
	rule.Business = &wrappers.StringValue{Value: "polaris"}
	rule.Owners = &wrappers.StringValue{Value: "polaris"}

	marshaler := &jsonpb.Marshaler{}
	ruleStr, err := marshaler.MarshalToString(rule)
	assert.Nil(t, err)
	assert.True(t, len(ruleStr) > 0)
}

/**
 * @brief 测试创建熔断规则
 */
func TestCreateCircuitBreaker(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	t.Run("正常创建熔断规则，返回成功", func(t *testing.T) {
		circuitBreakerReq, circuitBreakerResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(circuitBreakerResp.GetId().GetValue(), circuitBreakerResp.GetVersion().GetValue())
		checkCircuitBreaker(t, circuitBreakerReq, circuitBreakerReq, circuitBreakerResp)
	})

	t.Run("重复创建熔断规则，返回错误", func(t *testing.T) {
		_, circuitBreakerResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(circuitBreakerResp.GetId().GetValue(), circuitBreakerResp.GetVersion().GetValue())

		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{circuitBreakerResp}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则，删除，再创建，返回成功", func(t *testing.T) {
		_, circuitBreakerResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(circuitBreakerResp.GetId().GetValue(), circuitBreakerResp.GetVersion().GetValue())
		discoverSuit.deleteCircuitBreaker(t, circuitBreakerResp)

		newCircuitBreakerReq, newCircuitBreakerResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		checkCircuitBreaker(t, newCircuitBreakerReq, newCircuitBreakerReq, newCircuitBreakerResp)
		discoverSuit.cleanCircuitBreaker(newCircuitBreakerResp.GetId().GetValue(), newCircuitBreakerResp.GetVersion().GetValue())
	})

	t.Run("创建熔断规则时，没有传递负责人，返回错误", func(t *testing.T) {
		circuitBreaker := &apifault.CircuitBreaker{}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{circuitBreaker}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则时，没有传递规则名，返回错误", func(t *testing.T) {
		circuitBreaker := &apifault.CircuitBreaker{
			Namespace: utils.NewStringValue(service.DefaultNamespace),
			Owners:    utils.NewStringValue("test"),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{circuitBreaker}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则时，没有传递命名空间，返回错误", func(t *testing.T) {
		circuitBreaker := &apifault.CircuitBreaker{
			Name:   utils.NewStringValue("name-test-1"),
			Owners: utils.NewStringValue("test"),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{circuitBreaker}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("并发创建熔断规则，返回成功", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i < 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				_, circuitBreakerResp := discoverSuit.createCommonCircuitBreaker(t, index)
				discoverSuit.cleanCircuitBreaker(circuitBreakerResp.GetId().GetValue(), circuitBreakerResp.GetVersion().GetValue())
			}(i)
		}
		wg.Wait()
	})
}

/**
 * @brief 测试创建熔断规则版本
 */
func TestCreateCircuitBreakerVersion(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
	defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

	t.Run("正常创建熔断规则版本", func(t *testing.T) {
		cbVersionReq, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
		checkCircuitBreaker(t, cbVersionReq, cbVersionReq, cbVersionResp)
	})

	t.Run("传递id，正常创建熔断规则版本", func(t *testing.T) {
		cbVersionReq := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: utils.NewStringValue("test"),
			Token:   cbResp.GetToken(),
		}

		resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbVersionReq})
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		cbVersionResp := resp.Responses[0].GetCircuitBreaker()

		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		checkCircuitBreaker(t, cbVersionReq, cbVersionReq, cbVersionResp)
	})

	t.Run("传递name和namespace，正常创建熔断规则版本", func(t *testing.T) {
		cbVersionReq := &apifault.CircuitBreaker{
			Version:   utils.NewStringValue("test"),
			Name:      cbResp.GetName(),
			Namespace: cbResp.GetNamespace(),
			Token:     cbResp.GetToken(),
		}

		resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbVersionReq})
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		cbVersionResp := resp.Responses[0].GetCircuitBreaker()

		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		checkCircuitBreaker(t, cbVersionReq, cbVersionReq, cbVersionResp)
	})

	t.Run("创建熔断规则版本，删除，再创建，返回成功", func(t *testing.T) {
		cbVersionReq, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		discoverSuit.deleteCircuitBreaker(t, cbVersionResp)
		cbVersionReq, cbVersionResp = discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		checkCircuitBreaker(t, cbVersionReq, cbVersionReq, cbVersionResp)
	})

	t.Run("为不存在的熔断规则创建版本，返回错误", func(t *testing.T) {
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 1)
		discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		version := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: utils.NewStringValue("test"),
			Token:   cbResp.GetToken(),
			Owners:  cbResp.GetOwners(),
		}

		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{version}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建master版本的熔断规则，返回错误", func(t *testing.T) {
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbResp}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则版本时，没有传递version，返回错误", func(t *testing.T) {
		version := &apifault.CircuitBreaker{
			Id:     cbResp.GetId(),
			Token:  cbResp.GetToken(),
			Owners: cbResp.GetOwners(),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{version}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则版本时，没有传递token，返回错误", func(t *testing.T) {
		version := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: cbResp.GetVersion(),
			Owners:  cbResp.GetOwners(),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{version}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则版本时，没有传递name，返回错误", func(t *testing.T) {
		version := &apifault.CircuitBreaker{
			Version:   cbResp.GetVersion(),
			Token:     cbResp.GetToken(),
			Namespace: cbResp.GetNamespace(),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{version}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建熔断规则版本时，没有传递namespace，返回错误", func(t *testing.T) {
		version := &apifault.CircuitBreaker{
			Version: cbResp.GetVersion(),
			Token:   cbResp.GetToken(),
			Name:    cbResp.GetName(),
		}
		if resp := discoverSuit.DiscoverServer().CreateCircuitBreakerVersions(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{version}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("并发创建同一个规则的多个版本，返回成功", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 0; i <= 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				cbVersionReq, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, index)
				checkCircuitBreaker(t, cbVersionReq, cbVersionReq, cbVersionResp)
				defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
			}(i)
		}
		wg.Wait()
		t.Log("pass")
	})
}

/**
 * @brief 删除熔断规则
 */
func Test_DeleteCircuitBreaker(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	getCircuitBreakerVersions := func(t *testing.T, id string, expectNum uint32) {
		filters := map[string]string{
			"id": id,
		}
		resp := discoverSuit.DiscoverServer().GetCircuitBreakerVersions(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatal("error")
		}
		if resp.GetAmount().GetValue() != expectNum {
			t.Fatalf("error, actual num is %d, expect num is %d", resp.GetAmount().GetValue(), expectNum)
		} else {
			t.Log("pass")
		}
	}

	t.Run("根据name和namespace删除master版本的熔断规则", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		for i := 1; i <= 10; i++ {
			_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, i)
			defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
		}

		rule := &apifault.CircuitBreaker{
			Version:   cbResp.GetVersion(),
			Name:      cbResp.GetName(),
			Namespace: cbResp.GetNamespace(),
			Token:     cbResp.GetToken(),
		}

		discoverSuit.deleteCircuitBreaker(t, rule)
		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 10)
	})

	t.Run("删除master版本的熔断规则", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		for i := 1; i <= 10; i++ {
			_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, i)
			defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
		}

		discoverSuit.deleteCircuitBreaker(t, cbResp)
		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 10)
	})

	t.Run("删除非master版本的熔断规则", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		// 创建熔断规则版本
		for i := 1; i <= 10; i++ {
			_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, i)
			defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
		}

		// 删除特定版本的熔断规则
		discoverSuit.deleteCircuitBreaker(t, cbVersionResp)

		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 1+10)
	})

	t.Run("根据name和namespace删除非master版本的熔断规则", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		// 创建熔断规则版本
		for i := 1; i <= 10; i++ {
			_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, i)
			defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())
		}

		// 删除特定版本的熔断规则
		rule := &apifault.CircuitBreaker{
			Version:   cbVersionResp.GetVersion(),
			Name:      cbVersionResp.GetName(),
			Namespace: cbVersionResp.GetNamespace(),
			Token:     cbVersionResp.GetToken(),
		}
		discoverSuit.deleteCircuitBreaker(t, rule)

		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 1+10)
	})

	t.Run("删除不存在的熔断规则，返回成功", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		discoverSuit.deleteCircuitBreaker(t, cbResp)
		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 0)
	})

	t.Run("删除熔断规则时，没有传递token，返回错误", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		rule := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: cbResp.GetVersion(),
		}

		if resp := discoverSuit.DiscoverServer().DeleteCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{rule}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("删除熔断规则时，没有传递name和id，返回错误", func(t *testing.T) {
		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		rule := &apifault.CircuitBreaker{
			Version:   cbResp.GetVersion(),
			Namespace: cbResp.GetNamespace(),
			Token:     cbResp.GetToken(),
		}

		if resp := discoverSuit.DiscoverServer().DeleteCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{rule}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("删除已发布的规则，返回错误", func(t *testing.T) {
		// 创建服务
		_, serviceResp := discoverSuit.createCommonService(t, 0)
		defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		// 发布熔断规则
		discoverSuit.releaseCircuitBreaker(t, cbVersionResp, serviceResp)
		defer discoverSuit.cleanCircuitBreakerRelation(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue(),
			cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		// // 删除master版本
		// if resp := discoverSuit.DiscoverServer().DeleteCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbResp}); !respSuccess(resp) {
		// 	t.Logf("pass: %s", resp.GetInfo().GetValue())
		// } else {
		// 	t.Fatalf("error : %s", resp.GetInfo().GetValue())
		// }

		// 删除其他版本
		if resp := discoverSuit.DiscoverServer().DeleteCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbVersionResp}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("创建多个版本的规则，并发布其中一个规则，删除未发布规则，可以正常删除", func(t *testing.T) {
		// 创建服务
		_, serviceResp := discoverSuit.createCommonService(t, 0)
		defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

		// 创建熔断规则
		_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
		defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

		// 创建熔断规则版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		// 创建熔断规则版本
		_, newCbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 1)
		defer discoverSuit.cleanCircuitBreaker(newCbVersionResp.GetId().GetValue(), newCbVersionResp.GetVersion().GetValue())

		// 发布熔断规则
		discoverSuit.releaseCircuitBreaker(t, cbVersionResp, serviceResp)
		defer discoverSuit.cleanCircuitBreakerRelation(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue(),
			cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		discoverSuit.deleteCircuitBreaker(t, newCbVersionResp)
		getCircuitBreakerVersions(t, cbResp.GetId().GetValue(), 1+1)
	})

	t.Run("并发删除熔断规则，可以正常删除", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 1; i <= 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()
				_, cbResp := discoverSuit.createCommonCircuitBreaker(t, index)
				defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())
				discoverSuit.deleteCircuitBreaker(t, cbResp)
			}(i)
		}
		wg.Wait()
		t.Log("pass")
	})
}

/**
 * @brief 测试更新熔断规则
 */
func TestUpdateCircuitBreaker(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	// 创建熔断规则
	_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
	defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

	t.Run("更新master版本的熔断规则，返回成功", func(t *testing.T) {
		cbResp.Inbounds = []*apifault.CbRule{}
		discoverSuit.updateCircuitBreaker(t, cbResp)

		filters := map[string]string{
			"id":      cbResp.GetId().GetValue(),
			"version": cbResp.GetVersion().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreaker(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatal("error")
		}
		checkCircuitBreaker(t, cbResp, cbResp, resp.GetConfigWithServices()[0].GetCircuitBreaker())
	})

	t.Run("没有更新任何字段，返回不需要更新", func(t *testing.T) {
		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbResp}); respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("没有传递任何可更新的字段，返回不需要更新", func(t *testing.T) {
		rule := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: cbResp.GetVersion(),
			Token:   cbResp.GetToken(),
		}
		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{rule}); respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("负责人为空，返回错误", func(t *testing.T) {
		rule := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: cbResp.GetVersion(),
			Token:   cbResp.GetToken(),
			Owners:  utils.NewStringValue(""),
		}
		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{rule}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("更新其他版本的熔断规则，返回错误", func(t *testing.T) {
		// 创建熔断规则版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbVersionResp}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("更新不存在的熔断规则，返回错误", func(t *testing.T) {
		discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())
		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{cbResp}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("更新熔断规则时，没有传递token，返回错误", func(t *testing.T) {
		rule := &apifault.CircuitBreaker{
			Id:      cbResp.GetId(),
			Version: cbResp.GetVersion(),
		}
		if resp := discoverSuit.DiscoverServer().UpdateCircuitBreakers(discoverSuit.DefaultCtx, []*apifault.CircuitBreaker{rule}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("并发更新熔断规则时,可以正常更新", func(t *testing.T) {
		var wg sync.WaitGroup
		errs := make(chan error)
		for i := 1; i <= 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// 创建熔断规则
				_, cbResp := discoverSuit.createCommonCircuitBreaker(t, index)
				defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

				cbResp.Owners = utils.NewStringValue(fmt.Sprintf("test-owner-%d", index))

				discoverSuit.updateCircuitBreaker(t, cbResp)

				filters := map[string]string{
					"id":      cbResp.GetId().GetValue(),
					"version": cbResp.GetVersion().GetValue(),
				}
				resp := discoverSuit.DiscoverServer().GetCircuitBreaker(context.Background(), filters)
				if !respSuccess(resp) {
					errs <- fmt.Errorf("error : %v", resp)
					return
				}

				if len(resp.GetConfigWithServices()) != 1 {
					panic(errors.WithStack(fmt.Errorf("%#v", resp)))
				}

				checkCircuitBreaker(t, cbResp, cbResp, resp.GetConfigWithServices()[0].GetCircuitBreaker())
			}(i)
		}
		wg.Wait()

		select {
		case err := <-errs:
			if err != nil {
				t.Fatal(err)
			}
		default:
			return
		}
	})
}

/**
 * @brief 测试解绑熔断规则
 */
func TestUnBindCircuitBreaker(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	// 创建服务
	_, serviceResp := discoverSuit.createCommonService(t, 0)
	defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

	// 创建熔断规则
	_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
	defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

	// 创建熔断规则的版本
	_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 0)
	defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

	t.Run("解绑规则时没有传递token，返回错误", func(t *testing.T) {
		oldCtx := discoverSuit.DefaultCtx
		discoverSuit.DefaultCtx = context.Background()

		defer func() {
			discoverSuit.DefaultCtx = oldCtx
		}()

		unbind := &apiservice.ConfigRelease{
			Service: &apiservice.Service{
				Name:      serviceResp.GetName(),
				Namespace: serviceResp.GetNamespace(),
			},
			CircuitBreaker: cbVersionResp,
		}

		if resp := discoverSuit.DiscoverServer().UnBindCircuitBreakers(discoverSuit.DefaultCtx, []*apiservice.ConfigRelease{unbind}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("解绑服务不存在的熔断规则，返回错误", func(t *testing.T) {
		_, serviceResp := discoverSuit.createCommonService(t, 1)
		discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

		unbind := &apiservice.ConfigRelease{
			Service:        serviceResp,
			CircuitBreaker: cbVersionResp,
		}

		if resp := discoverSuit.DiscoverServer().UnBindCircuitBreakers(discoverSuit.DefaultCtx, []*apiservice.ConfigRelease{unbind}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("解绑规则不存在的熔断规则，返回错误", func(t *testing.T) {
		// 创建熔断规则的版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, 1)
		discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		unbind := &apiservice.ConfigRelease{
			Service:        serviceResp,
			CircuitBreaker: cbVersionResp,
		}

		if resp := discoverSuit.DiscoverServer().UnBindCircuitBreakers(discoverSuit.DefaultCtx, []*apiservice.ConfigRelease{unbind}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("解绑master版本的熔断规则，返回错误", func(t *testing.T) {
		unbind := &apiservice.ConfigRelease{
			Service:        serviceResp,
			CircuitBreaker: cbResp,
		}

		if resp := discoverSuit.DiscoverServer().UnBindCircuitBreakers(discoverSuit.DefaultCtx, []*apiservice.ConfigRelease{unbind}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("解绑熔断规则时没有传递name，返回错误", func(t *testing.T) {
		unbind := &apiservice.ConfigRelease{
			Service: serviceResp,
			CircuitBreaker: &apifault.CircuitBreaker{
				Version:   cbVersionResp.GetVersion(),
				Namespace: cbVersionResp.GetNamespace(),
			},
		}

		if resp := discoverSuit.DiscoverServer().UnBindCircuitBreakers(discoverSuit.DefaultCtx, []*apiservice.ConfigRelease{unbind}); !respSuccess(resp) {
			t.Logf("pass: %s", resp.GetInfo().GetValue())
		} else {
			t.Fatal("error")
		}
	})

	t.Run("并发解绑熔断规则", func(t *testing.T) {
		var wg sync.WaitGroup
		for i := 1; i <= 50; i++ {
			wg.Add(1)
			go func(index int) {
				defer wg.Done()

				// 创建服务
				_, serviceResp := discoverSuit.createCommonService(t, index)
				defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

				// 发布熔断规则
				discoverSuit.releaseCircuitBreaker(t, cbVersionResp, serviceResp)
				defer discoverSuit.cleanCircuitBreakerRelation(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue(),
					cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

				discoverSuit.unBindCircuitBreaker(t, cbVersionResp, serviceResp)
			}(i)
		}
		wg.Wait()
		t.Log("pass")
	})
}

/**
 * @brief 测试查询熔断规则
 */
func TestGetCircuitBreaker(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	versionNum := 10
	serviceNum := 2
	releaseVersion := &apifault.CircuitBreaker{}
	deleteVersion := &apifault.CircuitBreaker{}
	svc := &apiservice.Service{}

	// 创建熔断规则
	_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
	defer discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

	// 创建熔断规则版本
	for i := 1; i <= versionNum; i++ {
		// 创建熔断规则的版本
		_, cbVersionResp := discoverSuit.createCommonCircuitBreakerVersion(t, cbResp, i)
		defer discoverSuit.cleanCircuitBreaker(cbVersionResp.GetId().GetValue(), cbVersionResp.GetVersion().GetValue())

		if i == 5 {
			releaseVersion = cbVersionResp
		}

		if i == versionNum {
			deleteVersion = cbVersionResp
		}
	}

	// 删除一个版本的熔断规则
	discoverSuit.deleteCircuitBreaker(t, deleteVersion)

	// 发布熔断规则
	for i := 1; i <= serviceNum; i++ {
		_, serviceResp := discoverSuit.createCommonService(t, i)
		if i == 1 {
			svc = serviceResp
		}
		defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

		discoverSuit.releaseCircuitBreaker(t, releaseVersion, serviceResp)
		defer discoverSuit.cleanCircuitBreakerRelation(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue(),
			releaseVersion.GetId().GetValue(), releaseVersion.GetVersion().GetValue())
	}

	t.Run("测试获取熔断规则的所有版本", func(t *testing.T) {
		filters := map[string]string{
			"id": cbResp.GetId().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreakerVersions(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != resp.GetSize().GetValue() ||
			resp.GetSize().GetValue() != uint32(versionNum) || len(resp.GetConfigWithServices()) != versionNum {
			t.Fatalf("amount is %d, size is %d, num is %d, expect num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), len(resp.GetConfigWithServices()), versionNum)
		}
		t.Logf("pass: num is %d", resp.GetSize().GetValue())
	})

	t.Run("测试获取熔断规则创建过的版本", func(t *testing.T) {
		filters := map[string]string{
			"id": cbResp.GetId().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetReleaseCircuitBreakers(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != resp.GetSize().GetValue() ||
			resp.GetSize().GetValue() != uint32(serviceNum) {
			t.Fatalf("amount is %d, size is %d, expect num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), versionNum)
		}
		t.Logf("pass: num is %d", resp.GetSize().GetValue())
	})

	t.Run("测试获取指定版本的熔断规则", func(t *testing.T) {
		filters := map[string]string{
			"id":      releaseVersion.GetId().GetValue(),
			"version": releaseVersion.GetVersion().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreaker(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		checkCircuitBreaker(t, releaseVersion, cbResp, resp.GetConfigWithServices()[0].GetCircuitBreaker())
	})

	t.Run("根据服务获取绑定的熔断规则", func(t *testing.T) {
		filters := map[string]string{
			"service":   svc.GetName().GetValue(),
			"namespace": svc.GetNamespace().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreakerByService(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		checkCircuitBreaker(t, releaseVersion, cbResp, resp.GetConfigWithServices()[0].GetCircuitBreaker())
	})
}

/**
 * @brief 测试查询熔断规则
 */
func TestGetCircuitBreaker2(t *testing.T) {

	discoverSuit := &DiscoverTestSuit{}
	if err := discoverSuit.Initialize(); err != nil {
		t.Fatal(err)
	}
	defer discoverSuit.Destroy()

	// 创建服务
	_, serviceResp := discoverSuit.createCommonService(t, 0)
	defer discoverSuit.cleanServiceName(serviceResp.GetName().GetValue(), serviceResp.GetNamespace().GetValue())

	// 创建熔断规则
	_, cbResp := discoverSuit.createCommonCircuitBreaker(t, 0)
	discoverSuit.cleanCircuitBreaker(cbResp.GetId().GetValue(), cbResp.GetVersion().GetValue())

	t.Run("熔断规则不存在，测试获取所有版本", func(t *testing.T) {
		filters := map[string]string{
			"id": cbResp.GetId().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreakerVersions(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != 0 || resp.GetSize().GetValue() != 0 ||
			len(resp.GetConfigWithServices()) != 0 {
			t.Fatalf("amount is %d, size is %d, num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), len(resp.GetConfigWithServices()))
		}
		t.Logf("pass: resp is %+v, configServices is %+v", resp, resp.GetConfigWithServices())
	})

	t.Run("熔断规则不存在，测试获取所有创建过的版本", func(t *testing.T) {
		filters := map[string]string{
			"id": cbResp.GetId().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetReleaseCircuitBreakers(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != 0 || resp.GetSize().GetValue() != 0 ||
			len(resp.GetConfigWithServices()) != 0 {
			t.Fatalf("amount is %d, size is %d, num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), len(resp.GetConfigWithServices()))
		}
		t.Logf("pass: resp is %+v, configServices is %+v", resp, resp.GetConfigWithServices())
	})

	t.Run("熔断规则不存在，测试获取指定版本的熔断规则", func(t *testing.T) {
		filters := map[string]string{
			"id":      cbResp.GetId().GetValue(),
			"version": cbResp.GetVersion().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreaker(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != 0 || resp.GetSize().GetValue() != 0 ||
			len(resp.GetConfigWithServices()) != 0 {
			t.Fatalf("amount is %d, size is %d, num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), len(resp.GetConfigWithServices()))
		}
		t.Logf("pass: resp is %+v, configServices is %+v", resp, resp.GetConfigWithServices())
	})

	t.Run("服务未绑定熔断规则，获取熔断规则", func(t *testing.T) {
		filters := map[string]string{
			"service":   serviceResp.GetName().GetValue(),
			"namespace": serviceResp.GetNamespace().GetValue(),
		}

		resp := discoverSuit.DiscoverServer().GetCircuitBreakerByService(context.Background(), filters)
		if !respSuccess(resp) {
			t.Fatalf("error: %s", resp.GetInfo().GetValue())
		}
		if resp.GetAmount().GetValue() != 0 || resp.GetSize().GetValue() != 0 ||
			len(resp.GetConfigWithServices()) != 0 {
			t.Fatalf("amount is %d, size is %d, num is %d", resp.GetAmount().GetValue(),
				resp.GetSize().GetValue(), len(resp.GetConfigWithServices()))
		}
		t.Logf("pass: resp is %+v, configServices is %+v", resp, resp.GetConfigWithServices())
	})
}
