/*
Copyright 2023 The Volcano Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package vgpu

import (
	"context"
	"errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGetGPUMemoryOfPod(t *testing.T) {
	testCases := []struct {
		name string
		pod  *v1.Pod
		want uint
	}{
		{
			name: "GPUs required only in Containers",
			pod: &v1.Pod{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("3000"),
								},
							},
						},
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("3"),
									VolcanoVGPUMemory: resource.MustParse("5000"),
								},
							},
						},
					},
				},
			},
			want: 4,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got := resourcereqs(tc.pod)
			if got[0].Nums != 1 || got[0].Memreq != 3000 || got[1].Nums != 3 || got[1].Memreq != 5000 {
				t.Errorf("unexpected result, got: %v", got)
			}
		})
	}
}

func Test_NewGPUDevices(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu_node",
			Annotations: map[string]string{
				"volcano.sh/node-vgpu-register":  "GPU-d8794152-5506-fe60-be38-c6ff3d35dbf4,10,12288,NVIDIA-NVIDIA TITAN Xp,false:",
				"volcano.sh/node-vgpu-handshake": "Requesting_" + time.Now().Add(time.Second*100).Format("2006.01.02 15:04:05"),
			},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
			Allocatable: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
		},
	}
	devices := NewGPUDevices("vgpu_node", node)
	t.Logf("%+v", devices)
	if devices.Name != "vgpu_node" {
		t.Errorf("unexpected result, got: %v", devices)
	}
	if len(devices.Device) != 1 && devices.Device[0].Number != 10 && devices.Device[0].Memory != 12288 {
		t.Errorf("unexpected result, got: %v", devices)
	}
}

func Test_FilterNode(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu_node",
			Annotations: map[string]string{
				"volcano.sh/node-vgpu-register":  "GPU-d8794152-5506-fe60-be38-c6ff3d35dbf4,10,12288,NVIDIA-NVIDIA TITAN Xp,false:",
				"volcano.sh/node-vgpu-handshake": "Requesting_" + time.Now().Add(time.Second*100).Format("2006.01.02 15:04:05"),
			},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
			Allocatable: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
		},
	}
	tests := []struct {
		name    string
		req     v1.Pod
		want    bool
		wantErr error
	}{
		{
			name: "vGPU is filter pass",
			req: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("3000"),
									VolcanoVGPUCores:  resource.MustParse("50"),
								},
							},
						},
					},
				},
			},
			want:    true,
			wantErr: nil,
		},
		{
			name: "vGPU vgpu-cores more then vgpu number",
			req: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("3000"),
									VolcanoVGPUCores:  resource.MustParse("50"),
								},
							},
						},
						{
							Name: "test-container2",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("3000"),
									VolcanoVGPUCores:  resource.MustParse("60"),
								},
							},
						},
					},
				},
			},
			wantErr: errors.New("not enough gpu fitted on this node"),
			want:    false,
		},
		{
			name: "vGPU vgpu-memory more then vgpu memory",
			req: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name: "test-container",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("7000"),
									VolcanoVGPUCores:  resource.MustParse("50"),
								},
							},
						},
						{
							Name: "test-container2",
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("6000"),
									VolcanoVGPUCores:  resource.MustParse("50"),
								},
							},
						},
					},
				},
			},
			wantErr: errors.New("not enough gpu fitted on this node"),
			want:    false,
		},
	}
	VGPUEnable = true
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devices := NewGPUDevices("vgpu_node", node)
			got, err := devices.FilterNode(&tt.req)
			if err != nil || tt.wantErr != nil {
				if (err == nil && tt.wantErr != nil) ||
					(err != nil && tt.wantErr == nil) ||
					(err != nil && tt.wantErr != nil && err.Error() != tt.wantErr.Error()) {
					t.Errorf("unexpected result: %v, got: %v", tt.wantErr, err)
				}
			}
			if got != tt.want {
				t.Errorf("unexpected result, got: %v", got)
			}
		})
	}
}

func Test_Allocate(t *testing.T) {
	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: "vgpu_node",
			Annotations: map[string]string{
				"volcano.sh/node-vgpu-register":  "GPU-d8794152-5506-fe60-be38-c6ff3d35dbf4,10,12288,NVIDIA-NVIDIA TITAN Xp,false:",
				"volcano.sh/node-vgpu-handshake": "Requesting_" + time.Now().Add(time.Second*100).Format("2006.01.02 15:04:05"),
			},
		},
		Status: v1.NodeStatus{
			Capacity: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
			Allocatable: v1.ResourceList{
				"volcano.sh/vgpu-number": resource.MustParse("10"),
			},
		},
	}
	tests := []struct {
		name    string
		req     v1.Pod
		wantErr error
	}{
		{
			name: "vGPU is filter pass",
			req: v1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Resources: v1.ResourceRequirements{
								Limits: v1.ResourceList{
									VolcanoVGPUNumber: resource.MustParse("1"),
									VolcanoVGPUMemory: resource.MustParse("3000"),
									VolcanoVGPUCores:  resource.MustParse("50"),
								},
							},
						},
					},
				},
			},
			wantErr: nil,
		},
	}
	clientset := fake.NewSimpleClientset()
	clientset.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	VGPUEnable = true
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset.CoreV1().Pods("default").Create(context.Background(), &tt.req, metav1.CreateOptions{})
			devices := NewGPUDevices("vgpu_node", node)
			err := devices.Allocate(clientset, &tt.req)
			if err != nil || tt.wantErr != nil {
				if (err == nil && tt.wantErr != nil) ||
					(err != nil && tt.wantErr == nil) ||
					(err != nil && tt.wantErr != nil && err.Error() != tt.wantErr.Error()) {
					t.Errorf("unexpected result: %v, got: %v", tt.wantErr, err)
				}
			}
		})
	}
}
