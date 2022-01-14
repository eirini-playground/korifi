package repositories_test

import (
	"context"
	"time"

	. "github.com/onsi/gomega/gstruct"

	networkingv1alpha1 "code.cloudfoundry.org/cf-k8s-controllers/controllers/apis/networking/v1alpha1"

	. "code.cloudfoundry.org/cf-k8s-controllers/api/repositories"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("RouteRepository", func() {
	const domainName = "my-domain-name"

	var (
		testCtx    context.Context
		route1GUID string
		route2GUID string
		domainGUID string
		routeRepo  *RouteRepo
	)

	validateRoute := func(route RouteRecord, expectedRoute *networkingv1alpha1.CFRoute) {
		By("returning a routeRecord in the list for one of the created CRs", func() {
			Expect(route.GUID).To(Equal(expectedRoute.Name))
			Expect(route.Host).To(Equal(expectedRoute.Spec.Host))
			Expect(route.SpaceGUID).To(Equal(expectedRoute.Namespace))
			Expect(route.Path).To(Equal(expectedRoute.Spec.Path))
			Expect(route.Protocol).To(Equal(string(expectedRoute.Spec.Protocol)))
			Expect(route.Domain).To(Equal(DomainRecord{GUID: domainGUID}))

			Expect(route.Destinations).To(Equal([]DestinationRecord{
				{
					GUID:        expectedRoute.Spec.Destinations[0].GUID,
					AppGUID:     expectedRoute.Spec.Destinations[0].AppRef.Name,
					Port:        expectedRoute.Spec.Destinations[0].Port,
					ProcessType: expectedRoute.Spec.Destinations[0].ProcessType,
					Protocol:    expectedRoute.Spec.Destinations[0].Protocol,
				},
			}))

			validateTimestamp(route.CreatedAt, timeCheckThreshold*time.Second)
			validateTimestamp(route.UpdatedAt, timeCheckThreshold*time.Second)
		})
	}

	BeforeEach(func() {
		testCtx = context.Background()
		route1GUID = generateGUID()
		route2GUID = generateGUID()
		domainGUID = generateGUID()
		routeRepo = NewRouteRepo(k8sClient)
	})

	Describe("GetRoute", func() {
		var (
			cfRoute1 *networkingv1alpha1.CFRoute
			cfRoute2 *networkingv1alpha1.CFRoute
			cfDomain *networkingv1alpha1.CFDomain
		)

		BeforeEach(func() {
			cfDomain = &networkingv1alpha1.CFDomain{
				ObjectMeta: metav1.ObjectMeta{
					Name: domainGUID,
				},
				Spec: networkingv1alpha1.CFDomainSpec{
					Name: domainName,
				},
			}
			Expect(k8sClient.Create(testCtx, cfDomain)).To(Succeed())

			cfRoute1 = &networkingv1alpha1.CFRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      route1GUID,
					Namespace: "default",
				},
				Spec: networkingv1alpha1.CFRouteSpec{
					Host:     "my-subdomain-1",
					Path:     "",
					Protocol: "http",
					DomainRef: corev1.LocalObjectReference{
						Name: domainGUID,
					},
					Destinations: []networkingv1alpha1.Destination{
						{
							GUID: "destination-guid",
							Port: 8080,
							AppRef: corev1.LocalObjectReference{
								Name: "some-app-guid",
							},
							ProcessType: "web",
							Protocol:    "http1",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, cfRoute1)).To(Succeed())

			cfRoute2 = &networkingv1alpha1.CFRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      route2GUID,
					Namespace: "default",
				},
				Spec: networkingv1alpha1.CFRouteSpec{
					Host:     "my-subdomain-2",
					Path:     "",
					Protocol: "http",
					DomainRef: corev1.LocalObjectReference{
						Name: domainGUID,
					},
					Destinations: []networkingv1alpha1.Destination{},
				},
			}
			Expect(k8sClient.Create(testCtx, cfRoute2)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(testCtx, cfRoute1)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, cfRoute2)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, cfDomain)).To(Succeed())
		})

		When("multiple CFRoute resources exist", func() {
			It("fetches the CFRoute CR we're looking for", func() {
				route, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
				Expect(err).ToNot(HaveOccurred())

				Expect(route.GUID).To(Equal(cfRoute1.Name))
				Expect(route.Host).To(Equal(cfRoute1.Spec.Host))
				Expect(route.SpaceGUID).To(Equal(cfRoute1.Namespace))
				Expect(route.Path).To(Equal(cfRoute1.Spec.Path))
				Expect(route.Protocol).To(Equal(string(cfRoute1.Spec.Protocol)))

				By("returning a record with destinations that match the CFRoute CR", func() {
					Expect(route.Destinations).To(HaveLen(len(cfRoute1.Spec.Destinations)), "Route Record Destinations returned was not the correct length")
					destinationRecord := route.Destinations[0]
					Expect(destinationRecord.GUID).To(Equal(cfRoute1.Spec.Destinations[0].GUID))
					Expect(destinationRecord.AppGUID).To(Equal(cfRoute1.Spec.Destinations[0].AppRef.Name))
					Expect(destinationRecord.Port).To(Equal(cfRoute1.Spec.Destinations[0].Port))
					Expect(destinationRecord.ProcessType).To(Equal(cfRoute1.Spec.Destinations[0].ProcessType))
					Expect(destinationRecord.Protocol).To(Equal(cfRoute1.Spec.Destinations[0].Protocol))
				})

				By("returning a record where the CreatedAt and UpdatedAt match the CR creation time", func() {
					validateTimestamp(route.CreatedAt, timeCheckThreshold*time.Second)
					validateTimestamp(route.UpdatedAt, timeCheckThreshold*time.Second)
				})

				Expect(route.Domain).To(Equal(DomainRecord{GUID: domainGUID}))
			})
		})

		When("the CFRoute doesn't exist", func() {
			It("returns an error", func() {
				_, err := routeRepo.GetRoute(testCtx, authInfo, "non-existent-route-guid")
				Expect(err).To(MatchError(PermissionDeniedOrNotFoundError{}))
			})
		})

		When("multiple CFRoute resources exist across namespaces with the same name", func() {
			var (
				otherNamespaceGUID string
				otherNamespace     *corev1.Namespace

				cfRoute1A *networkingv1alpha1.CFRoute
			)

			BeforeEach(func() {
				// Create second namespace aside from default within which to create a duplicate route
				otherNamespaceGUID = generateGUID()
				otherNamespace = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: otherNamespaceGUID}}
				Expect(k8sClient.Create(testCtx, otherNamespace)).To(Succeed())

				cfRoute1A = &networkingv1alpha1.CFRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      route1GUID,
						Namespace: otherNamespaceGUID,
					},
					Spec: networkingv1alpha1.CFRouteSpec{
						Host:     "my-subdomain-1",
						Path:     "",
						Protocol: "http",
						DomainRef: corev1.LocalObjectReference{
							Name: domainGUID,
						},
					},
				}
				Expect(k8sClient.Create(testCtx, cfRoute1A)).To(Succeed())
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(testCtx, cfRoute1A)).To(Succeed())
				Expect(k8sClient.Delete(testCtx, otherNamespace)).To(Succeed())
			})

			It("returns an error", func() {
				// Looks like we can continue doing state-based setup for the time being
				// Assumption: when unit testing, we can ignore webhooks that might turn the uniqueness constraint into a race condition
				// If assumption is invalidated, we can implement the setup by mocking a fake client to return the non-unique ids

				_, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
				Expect(err).To(MatchError("duplicate route GUID exists"))
			})
		})
	})

	Describe("GetRouteList", func() {
		When("multiple CFRoutes exist", func() {
			var (
				cfRoute1 *networkingv1alpha1.CFRoute
				cfRoute2 *networkingv1alpha1.CFRoute
				cfDomain *networkingv1alpha1.CFDomain
			)

			BeforeEach(func() {
				cfDomain = &networkingv1alpha1.CFDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name: domainGUID,
					},
					Spec: networkingv1alpha1.CFDomainSpec{
						Name: domainName,
					},
				}
				Expect(k8sClient.Create(testCtx, cfDomain)).To(Succeed())

				cfRoute1 = &networkingv1alpha1.CFRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      route1GUID,
						Namespace: "default",
					},
					Spec: networkingv1alpha1.CFRouteSpec{
						Host:     "my-subdomain-1",
						Path:     "",
						Protocol: "http",
						DomainRef: corev1.LocalObjectReference{
							Name: domainGUID,
						},
						Destinations: []networkingv1alpha1.Destination{
							{
								GUID: "destination-guid",
								Port: 8080,
								AppRef: corev1.LocalObjectReference{
									Name: "some-app-guid",
								},
								ProcessType: "web",
								Protocol:    "http1",
							},
						},
					},
				}
				Expect(k8sClient.Create(testCtx, cfRoute1)).To(Succeed())

				cfRoute2 = &networkingv1alpha1.CFRoute{
					ObjectMeta: metav1.ObjectMeta{
						Name:      route2GUID,
						Namespace: "default",
					},
					Spec: networkingv1alpha1.CFRouteSpec{
						Host:     "my-subdomain-2",
						Path:     "/some/path",
						Protocol: "http",
						DomainRef: corev1.LocalObjectReference{
							Name: domainGUID,
						},
						Destinations: []networkingv1alpha1.Destination{
							{
								GUID: "destination-guid-2",
								Port: 8080,
								AppRef: corev1.LocalObjectReference{
									Name: "some-app-guid-2",
								},
								ProcessType: "web",
								Protocol:    "http1",
							},
						},
					},
				}
				Expect(k8sClient.Create(testCtx, cfRoute2)).To(Succeed())
			})

			AfterEach(func() {
				Expect(k8sClient.Delete(testCtx, cfRoute1)).To(Succeed())
				Expect(k8sClient.Delete(testCtx, cfRoute2)).To(Succeed())
				Expect(k8sClient.Delete(testCtx, cfDomain)).To(Succeed())
			})

			When("filters are not provided", func() {
				It("eventually returns a list of routeRecords for each CFRoute CR", func() {
					var routeRecords []RouteRecord
					Eventually(func() []RouteRecord {
						routeRecords, _ = routeRepo.ListRoutes(testCtx, authInfo, ListRoutesMessage{})
						return routeRecords
					}, timeCheckThreshold*time.Second).Should(ContainElements(
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfRoute1.Name)}),
						MatchFields(IgnoreExtras, Fields{"GUID": Equal(cfRoute2.Name)}),
					))

					var route1, route2 RouteRecord
					for _, routeRecord := range routeRecords {
						switch routeRecord.GUID {
						case cfRoute1.Name:
							route1 = routeRecord
						case cfRoute2.Name:
							route2 = routeRecord
						default:
						}
					}

					Expect(route1).NotTo(BeZero())
					Expect(route2).NotTo(BeZero())

					validateRoute(route1, cfRoute1)
					validateRoute(route2, cfRoute2)
				})
			})

			When("filters are provided", func() {
				var routeRecords []RouteRecord
				var message ListRoutesMessage

				JustBeforeEach(func() {
					Eventually(func() []RouteRecord {
						routeRecords, _ = routeRepo.ListRoutes(testCtx, authInfo, message)
						return routeRecords
					}, timeCheckThreshold*time.Second).ShouldNot(BeEmpty())
				})

				When("space_guid filters are provided", func() {
					BeforeEach(func() {
						message = ListRoutesMessage{SpaceGUIDs: []string{"default"}}
					})
					It("eventually returns a list of routeRecords for each CFRoute CR", func() {
						Expect(routeRecords).To(HaveLen(2))
					})
				})

				When("domain_guid filters are provided", func() {
					BeforeEach(func() {
						message = ListRoutesMessage{DomainGUIDs: []string{domainGUID}}
					})
					It("eventually returns a list of routeRecords for each CFRoute CR", func() {
						Expect(routeRecords).To(HaveLen(2))
					})
				})

				When("host filters are provided", func() {
					BeforeEach(func() {
						message = ListRoutesMessage{Hosts: []string{"my-subdomain-1"}}
					})
					It("eventually returns a list of routeRecords for one of the CFRoute CRs", func() {
						Expect(routeRecords).To(HaveLen(1))
					})
				})

				When("path filters are provided", func() {
					BeforeEach(func() {
						message = ListRoutesMessage{Paths: []string{"/some/path"}}
					})
					It("eventually returns a list of routeRecords for one of the CFRoute CRs", func() {
						Expect(routeRecords).To(HaveLen(1))
					})
				})

				When("app_guid filters are provided", func() {
					BeforeEach(func() {
						message = ListRoutesMessage{AppGUIDs: []string{"some-app-guid"}}
					})
					It("eventually returns a list of routeRecords for each CFRoute CR", func() {
						route1 := routeRecords[0]

						Expect(route1).NotTo(BeZero())
						validateRoute(route1, cfRoute1)
					})
				})
			})

			When("non-matching space_guid filters are provided", func() {
				It("eventually returns a list of routeRecords for each CFRoute CR", func() {
					message := ListRoutesMessage{SpaceGUIDs: []string{"something-not-matching"}}
					routeRecords, err := routeRepo.ListRoutes(testCtx, authInfo, message)
					Expect(err).ToNot(HaveOccurred())
					Expect(routeRecords).To(BeEmpty())
				})
			})

			When("non-matching domain_guid filters are provided", func() {
				It("eventually returns a list of routeRecords for each CFRoute CR", func() {
					message := ListRoutesMessage{DomainGUIDs: []string{"something-not-matching"}}
					routeRecords, err := routeRepo.ListRoutes(testCtx, authInfo, message)
					Expect(err).ToNot(HaveOccurred())
					Expect(routeRecords).To(BeEmpty())
				})
			})
		})

		When("no CFRoutes exist", Serial, func() {
			It("returns an empty list and no error", func() {
				Eventually(func() []RouteRecord {
					routeRecords, err := routeRepo.ListRoutes(testCtx, authInfo, ListRoutesMessage{})
					Expect(err).ToNot(HaveOccurred())
					return routeRecords
				}, timeCheckThreshold*time.Second).Should(BeEmpty())
			})
		})
	})

	Describe("GetRoutesForApp", func() {
		const (
			testNamespace = "default"
		)

		var (
			appGUID  string
			cfRoute1 *networkingv1alpha1.CFRoute
			cfRoute2 *networkingv1alpha1.CFRoute
		)

		BeforeEach(func() {
			appGUID = generateGUID()

			cfRoute1 = &networkingv1alpha1.CFRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      route1GUID,
					Namespace: testNamespace,
				},
				Spec: networkingv1alpha1.CFRouteSpec{
					Host:     "my-subdomain-1",
					Path:     "",
					Protocol: "http",
					DomainRef: corev1.LocalObjectReference{
						Name: domainGUID,
					},
					Destinations: []networkingv1alpha1.Destination{
						{
							GUID: "destination-guid",
							Port: 8080,
							AppRef: corev1.LocalObjectReference{
								Name: appGUID,
							},
							ProcessType: "web",
							Protocol:    "http1",
						},
					},
				},
			}
			Expect(k8sClient.Create(testCtx, cfRoute1)).To(Succeed())

			cfRoute2 = &networkingv1alpha1.CFRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      route2GUID,
					Namespace: testNamespace,
				},
				Spec: networkingv1alpha1.CFRouteSpec{
					Host:     "my-subdomain-2",
					Path:     "",
					Protocol: "http",
					DomainRef: corev1.LocalObjectReference{
						Name: domainGUID,
					},
					Destinations: []networkingv1alpha1.Destination{},
				},
			}
			Expect(k8sClient.Create(testCtx, cfRoute2)).To(Succeed())
		})

		AfterEach(func() {
			Expect(k8sClient.Delete(testCtx, cfRoute1)).To(Succeed())
			Expect(k8sClient.Delete(testCtx, cfRoute2)).To(Succeed())
		})

		When("multiple CFRoutes exist", func() {
			It("eventually returns a list of routeRecords for each CFRoute CR", func() {
				var routeRecords []RouteRecord
				Eventually(func() int {
					routeRecords, _ = routeRepo.ListRoutesForApp(testCtx, authInfo, appGUID, testNamespace)
					return len(routeRecords)
				}, timeCheckThreshold*time.Second).Should(Equal(1), "returned records count should equal number of created CRs with destinations to the App")

				By("returning a routeRecord in the list for one of the created CRs", func() {
					var route RouteRecord
					var found bool
					for _, routeRecord := range routeRecords {
						if routeRecord.GUID == cfRoute1.Name {
							found = true
							route = routeRecord
							break
						}
					}
					Expect(found).To(BeTrue(), "could not find matching record")

					By("returning a record with metadata fields from the CFRoute CR", func() {
						Expect(route.GUID).To(Equal(cfRoute1.Name))
						Expect(route.Host).To(Equal(cfRoute1.Spec.Host))
						Expect(route.SpaceGUID).To(Equal(cfRoute1.Namespace))
					})

					By("returning a record with spec fields from the CFRoute CR", func() {
						Expect(route.Path).To(Equal(cfRoute1.Spec.Path))
						Expect(route.Protocol).To(Equal(string(cfRoute1.Spec.Protocol)))
						Expect(route.Domain.GUID).To(Equal(cfRoute1.Spec.DomainRef.Name))
					})

					By("returning a record with destinations that match the CFRoute CR", func() {
						Expect(len(route.Destinations)).To(Equal(len(cfRoute1.Spec.Destinations)), "Route Record Destinations returned was not the correct length")
						destinationRecord := route.Destinations[0]
						Expect(destinationRecord.GUID).To(Equal(cfRoute1.Spec.Destinations[0].GUID))
						Expect(destinationRecord.AppGUID).To(Equal(cfRoute1.Spec.Destinations[0].AppRef.Name))
						Expect(destinationRecord.Port).To(Equal(cfRoute1.Spec.Destinations[0].Port))
						Expect(destinationRecord.ProcessType).To(Equal(cfRoute1.Spec.Destinations[0].ProcessType))
						Expect(destinationRecord.Protocol).To(Equal(cfRoute1.Spec.Destinations[0].Protocol))
					})

					By("returning a record where the CreatedAt and UpdatedAt match the CR creation time", func() {
						validateTimestamp(route.CreatedAt, timeCheckThreshold*time.Second)
						validateTimestamp(route.UpdatedAt, timeCheckThreshold*time.Second)
					})
				})
			})
		})

		When("no CFRoutes exist for the app", func() {
			It("returns an empty list and no error", func() {
				routeRecords, err := routeRepo.ListRoutesForApp(testCtx, authInfo, "i-dont-exist", testNamespace)
				Expect(err).ToNot(HaveOccurred())
				Expect(routeRecords).To(BeEmpty())
			})
		})
	})

	Describe("CreateRoute", func() {
		const (
			testNamespace = "default"
			testRouteHost = "test-route-host"
			testRoutePath = "/test/route/path"
		)

		When("route does not already exist", func() {
			var (
				createdRouteRecord RouteRecord
				createdRouteErr    error
			)

			BeforeEach(func() {
				cfDomain := &networkingv1alpha1.CFDomain{
					ObjectMeta: metav1.ObjectMeta{
						Name: domainGUID,
					},
					Spec: networkingv1alpha1.CFDomainSpec{
						Name: domainName,
					},
				}
				err := k8sClient.Create(context.Background(), cfDomain)
				Expect(err).NotTo(HaveOccurred())

				createRouteMessage := buildCreateRouteMessage(testRouteHost, testRoutePath, domainGUID, testNamespace)
				createdRouteRecord, createdRouteErr = routeRepo.CreateRoute(testCtx, authInfo, createRouteMessage)
				Expect(createdRouteErr).NotTo(HaveOccurred())
				route1GUID = createdRouteRecord.GUID
			})

			AfterEach(func() {
				Expect(cleanupRoute(k8sClient, testCtx, route1GUID, testNamespace)).To(Succeed())
				Expect(cleanupDomain(k8sClient, testCtx, domainGUID)).To(Succeed())
			})

			It("creates a new CFRoute CR successfully", func() {
				cfRouteLookupKey := types.NamespacedName{Name: route1GUID, Namespace: testNamespace}
				createdCFRoute := new(networkingv1alpha1.CFRoute)
				Eventually(func() string {
					err := k8sClient.Get(context.Background(), cfRouteLookupKey, createdCFRoute)
					if err != nil {
						return ""
					}
					return createdCFRoute.Name
				}, 10*time.Second, 250*time.Millisecond).Should(Equal(route1GUID))
			})

			It("returns an RouteRecord with matching fields", func() {
				Expect(createdRouteRecord.GUID).To(Equal(route1GUID), "Route GUID in record did not match input")
				Expect(createdRouteRecord.Host).To(Equal(testRouteHost), "Route Host in record did not match input")
				Expect(createdRouteRecord.Path).To(Equal(testRoutePath), "Route Path in record did not match input")
				Expect(createdRouteRecord.SpaceGUID).To(Equal(testNamespace), "Route Space GUID in record did not match input")
				Expect(createdRouteRecord.Domain).To(Equal(DomainRecord{GUID: domainGUID}), "Route Domain in record did not match created domain")

				validateTimestamp(createdRouteRecord.CreatedAt, 2*time.Second)
				validateTimestamp(createdRouteRecord.UpdatedAt, 2*time.Second)
			})
		})

		When("route creation fails", func() {
			When("namespace doesn't exist", func() {
				It("returns an error", func() {
					// TODO: improve this test so that the message is valid other than the namespace not existing
					_, err := routeRepo.CreateRoute(testCtx, authInfo, CreateRouteMessage{})
					Expect(err).To(MatchError("an empty namespace may not be set during creation"))
				})
			})
		})
	})

	Describe("GetOrCreateRoute", func() {
		const (
			testRouteHost = "test-route-host"
			testRoutePath = "/test/route/path"
		)

		var (
			testNamespace      string
			createRouteMessage CreateRouteMessage
		)

		BeforeEach(func() {
			testNamespace = generateGUID()
			testNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: testNamespace}}
			Expect(k8sClient.Create(testCtx, testNS)).To(Succeed())

			cfDomain := &networkingv1alpha1.CFDomain{
				ObjectMeta: metav1.ObjectMeta{
					Name: domainGUID,
				},
				Spec: networkingv1alpha1.CFDomainSpec{
					Name: domainName,
				},
			}
			err := k8sClient.Create(context.Background(), cfDomain)
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() { _ = cleanupDomain(k8sClient, testCtx, domainGUID) })

			createRouteMessage = buildCreateRouteMessage(testRouteHost, testRoutePath, domainGUID, testNamespace)
		})

		When("route does not already exist", func() {
			var (
				createdRouteRecord RouteRecord
				createdRouteErr    error
			)

			BeforeEach(func() {
				createdRouteRecord, createdRouteErr = routeRepo.GetOrCreateRoute(testCtx, authInfo, createRouteMessage)
				Expect(createdRouteErr).NotTo(HaveOccurred())
				route1GUID = createdRouteRecord.GUID
			})

			AfterEach(func() {
				Expect(cleanupRoute(k8sClient, testCtx, route1GUID, testNamespace)).To(Succeed())
			})

			It("creates a new CFRoute CR successfully", func() {
				cfRouteLookupKey := types.NamespacedName{Name: route1GUID, Namespace: testNamespace}
				createdCFRoute := new(networkingv1alpha1.CFRoute)
				Eventually(func() string {
					err := k8sClient.Get(context.Background(), cfRouteLookupKey, createdCFRoute)
					if err != nil {
						return ""
					}
					return createdCFRoute.Name
				}, 10*time.Second, 250*time.Millisecond).Should(Equal(route1GUID))
			})

			It("returns an RouteRecord with matching fields", func() {
				Expect(createdRouteRecord.GUID).To(Equal(route1GUID), "Route GUID in record did not match input")
				Expect(createdRouteRecord.Host).To(Equal(testRouteHost), "Route Host in record did not match input")
				Expect(createdRouteRecord.Path).To(Equal(testRoutePath), "Route Path in record did not match input")
				Expect(createdRouteRecord.SpaceGUID).To(Equal(testNamespace), "Route Space GUID in record did not match input")
				Expect(createdRouteRecord.Domain).To(Equal(DomainRecord{GUID: domainGUID}), "Route Domain in record did not match created domain")

				validateTimestamp(createdRouteRecord.CreatedAt, 2*time.Second)
				validateTimestamp(createdRouteRecord.UpdatedAt, 2*time.Second)
			})
		})

		When("the route already exists", func() {
			var existingRecord RouteRecord

			BeforeEach(func() {
				var err error
				existingRecord, err = routeRepo.CreateRoute(testCtx, authInfo, createRouteMessage)
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() {
					_ = cleanupRoute(k8sClient, testCtx, existingRecord.GUID, existingRecord.SpaceGUID)
				})
			})

			It("doesn't create a new route", func() {
				_, err := routeRepo.GetOrCreateRoute(testCtx, authInfo, createRouteMessage)
				Expect(err).NotTo(HaveOccurred())

				var routeList networkingv1alpha1.CFRouteList
				Expect(
					k8sClient.List(testCtx, &routeList, client.InNamespace(testNamespace)),
				).To(Succeed())

				Expect(routeList.Items).To(HaveLen(1))
			})

			It("returns the existing record", func() {
				returnedRecord, err := routeRepo.GetOrCreateRoute(testCtx, authInfo, createRouteMessage)
				Expect(err).NotTo(HaveOccurred())
				Expect(returnedRecord).To(Equal(existingRecord))
			})
		})

		When("route creation fails", func() {
			When("namespace doesn't exist", func() {
				It("returns an error", func() {
					// TODO: improve this test so that the message is valid other than the namespace not existing
					_, err := routeRepo.CreateRoute(testCtx, authInfo, CreateRouteMessage{})
					Expect(err).To(MatchError("an empty namespace may not be set during creation"))
				})
			})
		})
	})

	Describe("AddDestinationsToRoute", func() {
		const (
			testRouteHost = "test-route-host"
			testRoutePath = "/test/route/path"
		)

		var (
			testNamespace string
			namespace     *corev1.Namespace
		)

		BeforeEach(func() {
			testNamespace = "add-destinations-to-route-" + generateGUID()
			namespace = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: testNamespace,
				},
			}
			Expect(
				k8sClient.Create(testCtx, namespace),
			).To(Succeed())
			DeferCleanup(func() {
				_ = k8sClient.Delete(testCtx, namespace)
			})

			cfDomain := &networkingv1alpha1.CFDomain{
				ObjectMeta: metav1.ObjectMeta{Name: domainGUID},
			}
			Expect(
				k8sClient.Create(testCtx, cfDomain),
			).To(Succeed())
			DeferCleanup(func() {
				_ = cleanupDomain(k8sClient, testCtx, domainGUID)
			})
		})

		When("the route exists with no destinations", func() {
			BeforeEach(func() {
				cfRoute := initializeRouteCR(testRouteHost, testRoutePath, route1GUID, domainGUID, testNamespace)
				Expect(k8sClient.Create(testCtx, cfRoute)).To(Succeed())
				DeferCleanup(func() {
					_ = cleanupRoute(k8sClient, testCtx, route1GUID, testNamespace)
				})
			})

			When("route is updated to add new destinations", func() {
				var (
					appGUID1            string
					appGUID2            string
					destinationMessages []DestinationMessage
					patchedRouteRecord  RouteRecord
					addDestinationErr   error
				)

				BeforeEach(func() {
					appGUID1 = generateGUID()
					appGUID2 = generateGUID()
					destinationMessages = []DestinationMessage{
						{
							AppGUID:     appGUID1,
							ProcessType: "web",
							Port:        8080,
							Protocol:    "http1",
						},
						{
							AppGUID:     appGUID2,
							ProcessType: "worker",
							Port:        9000,
							Protocol:    "http1",
						},
					}

					routeRecord, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
					Expect(err).NotTo(HaveOccurred())

					// initialize a DestinationListMessage
					destinationListCreateMessage := initializeDestinationListMessage(routeRecord.GUID, routeRecord.SpaceGUID, routeRecord.Destinations, destinationMessages)
					patchedRouteRecord, addDestinationErr = routeRepo.AddDestinationsToRoute(testCtx, authInfo, destinationListCreateMessage)
					Expect(addDestinationErr).NotTo(HaveOccurred())
				})

				It("adds the destinations to CFRoute successfully", func() {
					cfRouteLookupKey := types.NamespacedName{Name: route1GUID, Namespace: testNamespace}
					createdCFRoute := new(networkingv1alpha1.CFRoute)
					Eventually(func() []networkingv1alpha1.Destination {
						err := k8sClient.Get(testCtx, cfRouteLookupKey, createdCFRoute)
						if err != nil {
							return nil
						}
						return createdCFRoute.Spec.Destinations
					}, 5*time.Second).Should(HaveLen(2), "could not retrieve cfRoute having exactly 2 destinations")

					Expect(createdCFRoute.Spec.Destinations).To(ConsistOf(
						MatchAllFields(
							Fields{
								"GUID": Not(BeEmpty()),
								"Port": Equal(8080),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID1,
								}),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID": Not(BeEmpty()),
								"Port": Equal(9000),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID2,
								}),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})

				It("returns RouteRecord with new destinations", func() {
					Expect(patchedRouteRecord.Destinations).To(ConsistOf(
						MatchAllFields(
							Fields{
								"GUID":        Not(BeEmpty()),
								"Port":        Equal(8080),
								"AppGUID":     Equal(appGUID1),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID":        Not(BeEmpty()),
								"Port":        Equal(9000),
								"AppGUID":     Equal(appGUID2),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})
			})

			When("the route destination has an invalid protocol", func() {
				It("returns an error", func() {
					appGUID := generateGUID()
					destinationMessages := []DestinationMessage{
						{
							AppGUID:     appGUID,
							ProcessType: "web",
							Port:        8080,
							Protocol:    "bad-protocol",
						},
					}

					routeRecord, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
					Expect(err).NotTo(HaveOccurred())

					// initialize a DestinationListMessage
					destinationListCreateMessage := initializeDestinationListMessage(routeRecord.GUID, routeRecord.SpaceGUID, routeRecord.Destinations, destinationMessages)
					_, addDestinationErr := routeRepo.AddDestinationsToRoute(testCtx, authInfo, destinationListCreateMessage)
					Expect(addDestinationErr.Error()).To(ContainSubstring("Unsupported value: \"bad-protocol\": supported values: \"http1\""))
				})
			})
		})

		When("the route exists with a destination", func() {
			var (
				routeDestination networkingv1alpha1.Destination
				destinationGUID  string
				appGUID          string
			)

			BeforeEach(func() {
				cfRoute := initializeRouteCR(testRouteHost, testRoutePath, route1GUID, domainGUID, testNamespace)

				destinationGUID = generateGUID()
				appGUID = generateGUID()
				routeDestination = networkingv1alpha1.Destination{
					GUID: destinationGUID,
					Port: 8000,
					AppRef: corev1.LocalObjectReference{
						Name: appGUID,
					},
					ProcessType: "web",
					Protocol:    "http1",
				}

				cfRoute.Spec.Destinations = []networkingv1alpha1.Destination{routeDestination}
				Expect(k8sClient.Create(testCtx, cfRoute)).To(Succeed())
				DeferCleanup(func() {
					_ = cleanupRoute(k8sClient, testCtx, route1GUID, testNamespace)
				})
			})

			When("the destinations are all new", func() {
				var (
					appGUID1            string
					appGUID2            string
					destinationMessages []DestinationMessage
					patchedRouteRecord  RouteRecord
					addDestinationErr   error
				)

				BeforeEach(func() {
					appGUID1 = generateGUID()
					appGUID2 = generateGUID()
					destinationMessages = []DestinationMessage{
						{
							AppGUID:     appGUID1,
							ProcessType: "web",
							Port:        8080,
							Protocol:    "http1",
						},
						{
							AppGUID:     appGUID2,
							ProcessType: "worker",
							Port:        9000,
							Protocol:    "http1",
						},
						// Duplicate dest that should be ignored
						{
							AppGUID:     appGUID2,
							ProcessType: "worker",
							Port:        9000,
							Protocol:    "http1",
						},
					}

					routeRecord, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
					Expect(err).NotTo(HaveOccurred())

					destinationListCreateMessage := initializeDestinationListMessage(routeRecord.GUID, routeRecord.SpaceGUID, routeRecord.Destinations, destinationMessages)
					patchedRouteRecord, addDestinationErr = routeRepo.AddDestinationsToRoute(testCtx, authInfo, destinationListCreateMessage)
					Expect(addDestinationErr).NotTo(HaveOccurred())
				})

				It("adds the destinations to CFRoute successfully", func() {
					testCtx = context.Background()
					cfRouteLookupKey := types.NamespacedName{Name: route1GUID, Namespace: testNamespace}
					createdCFRoute := new(networkingv1alpha1.CFRoute)
					Eventually(func() []networkingv1alpha1.Destination {
						err := k8sClient.Get(testCtx, cfRouteLookupKey, createdCFRoute)
						if err != nil {
							return nil
						}
						return createdCFRoute.Spec.Destinations
					}, 5*time.Second).Should(HaveLen(3))

					Expect(createdCFRoute.Spec.Destinations).To(ConsistOf(
						MatchAllFields(
							Fields{
								"GUID": Not(BeEmpty()),
								"Port": Equal(8080),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID1,
								}),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID": Not(BeEmpty()),
								"Port": Equal(9000),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID2,
								}),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID": Equal(destinationGUID),
								"Port": Equal(8000),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID,
								}),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})

				It("returns RouteRecord with new destinations", func() {
					Expect(patchedRouteRecord.Destinations).To(ConsistOf(
						MatchAllFields(
							Fields{
								"GUID":        Not(BeEmpty()),
								"Port":        Equal(8080),
								"AppGUID":     Equal(appGUID1),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID":        Not(BeEmpty()),
								"Port":        Equal(9000),
								"AppGUID":     Equal(appGUID2),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
						MatchAllFields(
							Fields{
								"GUID":        Equal(destinationGUID),
								"Port":        Equal(8000),
								"AppGUID":     Equal(appGUID),
								"ProcessType": Equal("web"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})
			})

			When("one of the destinations is already on the route", func() {
				var (
					appGUID2               string
					addDestinationMessages []DestinationMessage
					patchedRouteRecord     RouteRecord
				)

				BeforeEach(func() {
					appGUID2 = generateGUID()
					addDestinationMessages = []DestinationMessage{
						{
							AppGUID:     routeDestination.AppRef.Name,
							ProcessType: routeDestination.ProcessType,
							Port:        routeDestination.Port,
							Protocol:    routeDestination.Protocol,
						},
						{
							AppGUID:     appGUID2,
							ProcessType: "worker",
							Port:        9000,
							Protocol:    "http1",
						},
					}

					routeRecord, err := routeRepo.GetRoute(testCtx, authInfo, route1GUID)
					Expect(err).NotTo(HaveOccurred())

					destinationListCreateMessage := initializeDestinationListMessage(routeRecord.GUID, routeRecord.SpaceGUID, routeRecord.Destinations, addDestinationMessages)
					patchedRouteRecord, err = routeRepo.AddDestinationsToRoute(testCtx, authInfo, destinationListCreateMessage)
					Expect(err).NotTo(HaveOccurred())
				})

				It("adds only the new destination to CFRoute successfully", func() {
					testCtx = context.Background()
					cfRouteLookupKey := types.NamespacedName{Name: route1GUID, Namespace: testNamespace}
					createdCFRoute := new(networkingv1alpha1.CFRoute)
					Eventually(func() []networkingv1alpha1.Destination {
						err := k8sClient.Get(testCtx, cfRouteLookupKey, createdCFRoute)
						if err != nil {
							return nil
						}
						return createdCFRoute.Spec.Destinations
					}, 5*time.Second).Should(HaveLen(2))

					Expect(createdCFRoute.Spec.Destinations).To(ConsistOf(
						networkingv1alpha1.Destination{
							GUID:        routeDestination.GUID,
							AppRef:      corev1.LocalObjectReference{Name: routeDestination.AppRef.Name},
							ProcessType: routeDestination.ProcessType,
							Port:        routeDestination.Port,
							Protocol:    routeDestination.Protocol,
						},
						MatchAllFields(
							Fields{
								"GUID": Not(BeEmpty()),
								"Port": Equal(9000),
								"AppRef": Equal(corev1.LocalObjectReference{
									Name: appGUID2,
								}),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})

				It("returns RouteRecord with new destinations", func() {
					Expect(patchedRouteRecord.Destinations).To(ConsistOf(
						DestinationRecord{
							GUID:        routeDestination.GUID,
							AppGUID:     routeDestination.AppRef.Name,
							ProcessType: routeDestination.ProcessType,
							Port:        routeDestination.Port,
							Protocol:    routeDestination.Protocol,
						},
						MatchAllFields(
							Fields{
								"GUID":        Not(BeEmpty()),
								"Port":        Equal(9000),
								"AppGUID":     Equal(appGUID2),
								"ProcessType": Equal("worker"),
								"Protocol":    Equal("http1"),
							},
						),
					))
				})
			})
		})
	})
})

func initializeRouteCR(routeHost, routePath, routeGUID, domainGUID, spaceGUID string) *networkingv1alpha1.CFRoute {
	return &networkingv1alpha1.CFRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeGUID,
			Namespace: spaceGUID,
		},
		Spec: networkingv1alpha1.CFRouteSpec{
			Host: routeHost,
			Path: routePath,
			DomainRef: corev1.LocalObjectReference{
				Name: domainGUID,
			},
		},
	}
}

func initializeDestinationListMessage(routeGUID string, spaceGUID string, existingDestinations []DestinationRecord, addDestinations []DestinationMessage) AddDestinationsToRouteMessage {
	return AddDestinationsToRouteMessage{
		RouteGUID:            routeGUID,
		SpaceGUID:            spaceGUID,
		ExistingDestinations: existingDestinations,
		NewDestinations:      addDestinations,
	}
}

func buildCreateRouteMessage(routeHost, routePath, domainGUID, spaceGUID string) CreateRouteMessage {
	return CreateRouteMessage{
		Host:       routeHost,
		Path:       routePath,
		SpaceGUID:  spaceGUID,
		DomainGUID: domainGUID,
	}
}

func cleanupRoute(k8sClient client.Client, ctx context.Context, routeGUID, routeNamespace string) error {
	return k8sClient.Delete(ctx, &networkingv1alpha1.CFRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeGUID,
			Namespace: routeNamespace,
		},
	})
}

func cleanupDomain(k8sClient client.Client, ctx context.Context, domainGUID string) error {
	return k8sClient.Delete(ctx, &networkingv1alpha1.CFDomain{
		ObjectMeta: metav1.ObjectMeta{
			Name: domainGUID,
		},
	})
}

func validateTimestamp(timestamp string, interval time.Duration) {
	recordCreatedTime, err := time.Parse(TimestampFormat, timestamp)
	Expect(err).NotTo(HaveOccurred(), "There was an error converting the createRouteRecord CreatedTime to string")
	Expect(recordCreatedTime).To(BeTemporally("~", time.Now(), interval))
}
