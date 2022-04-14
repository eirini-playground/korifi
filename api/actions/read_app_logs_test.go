package actions_test

import (
	"context"
	"errors"
	"time"

	. "code.cloudfoundry.org/cf-k8s-controllers/api/actions"
	"code.cloudfoundry.org/cf-k8s-controllers/api/actions/fake"
	"code.cloudfoundry.org/cf-k8s-controllers/api/apierrors"
	"code.cloudfoundry.org/cf-k8s-controllers/api/authorization"
	"code.cloudfoundry.org/cf-k8s-controllers/api/payloads"
	"code.cloudfoundry.org/cf-k8s-controllers/api/repositories"
	"code.cloudfoundry.org/cf-k8s-controllers/tests/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	testLogCacheHandlerLoggerName = "TestLogCacheHandler"
)

var _ = Describe("ReadAppLogs", func() {
	const (
		appGUID   = "test-app-guid"
		buildGUID = "test-build-guid"

		spaceGUID = "test-space-guid"
	)

	var (
		appRepo   *fake.CFAppRepository
		buildRepo *fake.CFBuildRepository
		podRepo   *fake.PodRepository

		readAppLogsAction *ReadAppLogs

		buildLogs, appLogs []repositories.LogRecord

		authInfo       authorization.Info
		requestPayload payloads.LogRead

		returnedRecords []repositories.LogRecord
		returnedErr     error
	)

	BeforeEach(func() {
		appRepo = new(fake.CFAppRepository)
		buildRepo = new(fake.CFBuildRepository)
		podRepo = new(fake.PodRepository)

		readAppLogsAction = NewReadAppLogs(logf.Log.WithName(testLogCacheHandlerLoggerName), appRepo, buildRepo, podRepo)

		appRepo.GetAppReturns(repositories.AppRecord{
			GUID:      appGUID,
			Revision:  "1",
			SpaceGUID: spaceGUID,
		}, nil)

		buildRepo.GetLatestBuildByAppGUIDReturns(repositories.BuildRecord{
			GUID:    buildGUID,
			AppGUID: appGUID,
		}, nil)

		buildLogs = []repositories.LogRecord{
			{
				Message:   "BuildMessage1",
				Timestamp: time.Now().UnixNano(),
			},
			{
				Message:   "BuildMessage2",
				Timestamp: time.Now().UnixNano(),
			},
		}
		buildRepo.GetBuildLogsReturns(buildLogs, nil)

		time.Sleep(5 * time.Millisecond)

		appLogs = []repositories.LogRecord{
			{
				Message:   "AppMessage1",
				Timestamp: time.Now().UnixNano(),
			},
			{
				Message:   "AppMessage2",
				Timestamp: time.Now().UnixNano(),
			},
		}
		podRepo.GetRuntimeLogsForAppReturns(appLogs, nil)

		requestPayload = payloads.LogRead{
			StartTime:     nil,
			EndTime:       nil,
			EnvelopeTypes: nil,
			Limit:         nil,
			Descending:    nil,
		}
		authInfo = authorization.Info{Token: "a-token"}
	})

	JustBeforeEach(func() {
		returnedRecords, returnedErr = readAppLogsAction.Invoke(context.Background(), authInfo, appGUID, requestPayload)
	})

	It("sets the log limit to 100 when not specified", func() {
		Expect(podRepo.GetRuntimeLogsForAppCallCount()).To(BeNumerically(">=", 1))
		_, _, message := podRepo.GetRuntimeLogsForAppArgsForCall(0)
		Expect(message.Limit).To(Equal(int64(100)))
	})

	It("returns the list of build and app records", func() {
		Expect(returnedErr).NotTo(HaveOccurred())
		Expect(returnedRecords).To(Equal(append(buildLogs, appLogs...)))
	})

	When("GetApp returns a Forbidden error", func() {
		BeforeEach(func() {
			appRepo.GetAppReturns(repositories.AppRecord{}, apierrors.NewForbiddenError(errors.New("blah"), repositories.AppResourceType))
		})
		It("returns a NotFound error", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(matchers.WrapErrorAssignableToTypeOf(apierrors.NotFoundError{}))
		})
	})

	When("GetApp returns a random error", func() {
		var getAppError error
		BeforeEach(func() {
			getAppError = errors.New("blah")
			appRepo.GetAppReturns(repositories.AppRecord{}, getAppError)
		})
		It("returns the error transparently", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(Equal(getAppError))
		})
	})

	When("GetLatestBuildByAppGUIDReturns returns a Forbidden error", func() {
		BeforeEach(func() {
			buildRepo.GetLatestBuildByAppGUIDReturns(repositories.BuildRecord{}, apierrors.NewForbiddenError(errors.New("blah"), repositories.BuildResourceType))
		})
		It("returns a NotFound error", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(matchers.WrapErrorAssignableToTypeOf(apierrors.NotFoundError{}))
		})
	})

	When("GetLatestBuildByAppGUIDReturns returns a random error", func() {
		var getLatestBuildByAppGUID error
		BeforeEach(func() {
			getLatestBuildByAppGUID = errors.New("blah")
			buildRepo.GetLatestBuildByAppGUIDReturns(repositories.BuildRecord{}, getLatestBuildByAppGUID)
		})
		It("returns the error transparently", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(Equal(getLatestBuildByAppGUID))
		})
	})

	When("GetBuildLogsReturns returns an error", func() {
		var getBuildLogsReturns error
		BeforeEach(func() {
			getBuildLogsReturns = errors.New("blah")
			buildRepo.GetBuildLogsReturns(nil, getBuildLogsReturns)
		})
		It("returns the error transparently", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(Equal(getBuildLogsReturns))
		})
	})

	When("GetRuntimeLogsForAppReturns returns an error", func() {
		var getRuntimeLogsReturns error
		BeforeEach(func() {
			getRuntimeLogsReturns = errors.New("blah")
			podRepo.GetRuntimeLogsForAppReturns(nil, getRuntimeLogsReturns)
		})
		It("returns the error transparently", func() {
			Expect(returnedErr).To(HaveOccurred())
			Expect(returnedErr).To(Equal(getRuntimeLogsReturns))
		})
	})
})
