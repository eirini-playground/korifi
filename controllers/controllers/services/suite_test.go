package services_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestServicesControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Services Controllers Unit Test Suite")
}