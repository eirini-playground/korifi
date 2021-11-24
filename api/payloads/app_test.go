package payloads

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("AppList", func() {
	Describe("ToMessage", func() {
		Describe("names", func() {
			When("a single name is specified", func() {
				It("properly splits them in the message", func() {
					payload := AppList{Names: "example.com"}

					Expect(payload.ToMessage().Names).To(Equal([]string{"example.com"}))
				})
			})

			When("multiple names are specified", func() {
				It("properly splits them in the message and truncates whitespace", func() {
					payload := AppList{Names: " example.com, example.org ,cloudfoundry.org "}

					Expect(payload.ToMessage().Names).To(Equal([]string{"example.com", "example.org", "cloudfoundry.org"}))
				})
			})

			When("no names are specified", func() {
				It("sets Names to an empty array", func() {
					payload := AppList{}

					Expect(payload.ToMessage().Names).To(Equal([]string{}))
					Expect(len(payload.ToMessage().Names)).To(Equal(0))
				})
			})
		})

		Describe("space_guids", func() {
			When("a single space guid is specified", func() {
				It("properly splits them in the message", func() {
					payload := AppList{SpaceGuids: "f6dea88f-0781-4461-b8d9-09fd6f5a0f40"}

					Expect(payload.ToMessage().SpaceGuids).To(Equal([]string{"f6dea88f-0781-4461-b8d9-09fd6f5a0f40"}))
				})
			})

			When("multiple space guids are specified", func() {
				It("properly splits them in the message and truncates whitespace", func() {
					payload := AppList{SpaceGuids: " f6dea88f-0781-4461-b8d9-09fd6f5a0f40, ad0836b5-09f4-48c0-adb2-2c61e515562f ,6030b015-f003-4c9f-8bb4-1ed7ae3d3659 "}

					Expect(payload.ToMessage().SpaceGuids).To(Equal([]string{"f6dea88f-0781-4461-b8d9-09fd6f5a0f40", "ad0836b5-09f4-48c0-adb2-2c61e515562f", "6030b015-f003-4c9f-8bb4-1ed7ae3d3659"}))
				})
			})

			When("no space guids are specified", func() {
				It("sets SpaceGuids to an empty array", func() {
					payload := AppList{}

					Expect(payload.ToMessage().SpaceGuids).To(Equal([]string{}))
					Expect(len(payload.ToMessage().SpaceGuids)).To(Equal(0))
				})
			})
		})
	})
})