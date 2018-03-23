package plugin_test

import (
	. "github.com/arquillian/ike-prow-plugins/pkg/plugin/test-keeper/plugin"
	"github.com/arquillian/ike-prow-plugins/pkg/scm"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Test fileCategoryCounter features", func() {

	Context("Detecting tests within file changeset", func() {

		It("should accept changeset containing Java file set when based on predefined matchers", func() {
			// given
			changedFiles := changedFilesSet(
				"path/to/Anything.java",
				"path/to/page.html",
				"path/to/test/AnythingTestCase.java")

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should accept changeset containing Go tests using predefined matchers", func() {
			// given
			changedFiles := changedFilesSet(
				"pkg/plugin/test-keeper/plugin/test_checker.go",
				"path/to/golang/main_test.go")

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should accept changeset containing Go and Java tests using predefined matchers", func() {
			// given
			changedFiles := changedFilesSet(
				"path/to/JavaTest.java",
				"path/to/golang/main_test.go")

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should not accept changeset when files are not matching predefined language test patterns", func() {
			// given
			changedFiles := changedFilesSet(
				"path/to/Anything.java",
				"path/_test.go/page.html",
				"path/Test.java/js/something.in.js",
				"path/to/go/another_in.go")

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeFalse())
		})

		It("should not accept changeset when test files are in external dependency folders", func() {
			// given
			changedFiles := changedFilesSet(
				"node_modules/leftpad/dont_delete_me.spec.js",
				"vendor/github.com/test/repo/should_ignore_this_test.go")
			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeFalse())
		})

		It("should not try to detect any tests when change set is empty", func() {
			// given
			changedFiles := changedFilesSet()

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.Total).To(Equal(0))
			Expect(fileCategories.TestsExist()).To(BeFalse())
		})

		It("should accept changeset based on configured inclusion", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{Inclusion: `_test\.rb$`})

			changedFiles := changedFilesSet(
				"path/to/github_service.rb",
				"path/to/github_service_test.rb")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should accept changeset using inclusion in the configuration", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Inclusion: `(Test\.java|TestCase\.java|_test\.go)$`,
			})

			changedFiles := changedFilesSet(
				"path/to/JavaTest.java",
				"path/to/JavaTestCase.java",
				"path/to/JavaTestCase.java",
				"path/to/golang/main_test.go")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should accept changeset containing default exclusion such as documentation, ci and build files", func() {
			// given
			changedFiles := changedFilesSet(
				"path/to/README.adoc",
				"pom.xml",
				".travis.yml")

			fileCategoryCounter := FileCategoryCounter{Matcher: DefaultMatchers}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.OnlySkippedFiles()).To(BeTrue())
		})

		It("should accept changeset containing configured exclusion and one test matched by default inclusion", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Exclusion: `(\.txt|\.svg|\.png)$`,
			})

			changedFiles := changedFilesSet(
				"src/test/java/org/my/CoolTestCase.java",
				"path/to/README.txt",
				"meme.svg",
				"test.png")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

		It("should accept changeset containing configured exclusion", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Exclusion: `(\.txt|\.svg|\.png)$`,
			})

			changedFiles := changedFilesSet(
				"path/to/README.txt",
				"meme.svg",
				"test.png")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.OnlySkippedFiles()).To(BeTrue())
		})

		It("should accept changeset containing configured overlapping exclusion and inclusion", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Inclusion: `test\.txt`,
				Exclusion: `(\.txt|\.svg|\.png)$`,
			})

			changedFiles := changedFilesSet(
				"path/to/my_test.txt",
				"path/to/README.txt",
				"meme.svg",
				"test.png")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.OnlySkippedFiles()).To(BeTrue())
		})

		It("should accept changeset containing exclusion combined with default excluded files", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Exclusion: `.svg$`,
				Combine:   true,
			})

			changedFiles := changedFilesSet(
				"test.svg",
				"path/to/README.adoc",
				"pom.xml",
				".travis.yml")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.OnlySkippedFiles()).To(BeTrue())
		})

		It("should accept changeset containing inclusion combined with default included files", func() {
			// given
			matcher := LoadMatcher(TestKeeperConfiguration{
				Inclusion: `FunctionalTest.java$`,
				Combine:   true,
			})

			changedFiles := changedFilesSet(
				"src/test/com/acme/UnitTest.java",
				"src/test/com/acme/ServiceIT.java",
				"src/test/com/acme/FancyTestCase.java",
				"src/test/com/acme/AwesomeFunctionalTest.java")

			fileCategoryCounter := FileCategoryCounter{Matcher: matcher}

			// when
			fileCategories, err := fileCategoryCounter.Count(changedFiles)

			// then
			Ω(err).ShouldNot(HaveOccurred())
			Expect(fileCategories.TestsExist()).To(BeTrue())
		})

	})

})

func changedFilesSet(names ...string) []scm.ChangedFile {
	files := make([]scm.ChangedFile, 0, len(names))
	for _, name := range names {
		files = append(files, scm.ChangedFile{Name: name, Status: "added"})
	}
	return files
}