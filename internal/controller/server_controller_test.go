/*
Copyright 2025.

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

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	minecraftv1 "github.com/WangQiHao-Charlie/minecraft-operator/api/v1"
)

var _ = Describe("Server Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		server := &minecraftv1.Server{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Server")
			err := k8sClient.Get(ctx, typeNamespacedName, server)
			if err != nil && errors.IsNotFound(err) {
				resource := &minecraftv1.Server{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					Spec: minecraftv1.ServerSpec{
						HardwareResource: minecraftv1.HardwareSpec{
							StorageSize:      10,
							MemorySize:       2048,
							CPUCount:         1000,
							StorageClassName: "standard",
						},
						JavaVersion: "21",
						Minecraft: minecraftv1.MinecraftConfig{
							Eula:    "TRUE",
							Type:    "PAPER",
							Version: "1.21.1",
							Motd:    "hello world",
							Modpack: minecraftv1.ModpackSource{
								Type: "HTTP",
								URL:  "https://example.invalid/modpack.zip",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &minecraftv1.Server{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Server")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &ServerReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			By("creating dependent resources")
			configMap := &v1.ConfigMap{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resourceName + "-config", Namespace: "default"}, configMap)).To(Succeed())
			Expect(configMap.Data).To(HaveKeyWithValue("MOTD", "hello world"))

			statefulSet := &appsv1.StatefulSet{}
			Expect(k8sClient.Get(ctx, typeNamespacedName, statefulSet)).To(Succeed())
			Expect(statefulSet.Spec.Template.Annotations).To(HaveKey(podTemplateHashAnnotation))
		})
	})
})
