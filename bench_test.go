package tego

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/seeruk/tego/examples/shapes/shapes"
	"github.com/seeruk/tego/examples/shapes/shapespbv1"
	"google.golang.org/protobuf/proto"
)

const largeProjectBenchmarkScale = 50

var (
	benchmarkProjectProtoSink *shapespbv1.Project
	benchmarkProjectTegoSink  shapes.Project
	benchmarkBytesSink        []byte
)

type projectBenchmarkFixture struct {
	tego       shapes.Project
	proto      *shapespbv1.Project
	protoBytes []byte
	jsonBytes  []byte
	msgpBytes  []byte
}

func BenchmarkProjectEncoding(b *testing.B) {
	fixture := newProjectBenchmarkFixture(b)
	runProjectEncodingBenchmarks(b, fixture)
}

func BenchmarkLargeProjectEncoding(b *testing.B) {
	fixture := newLargeProjectBenchmarkFixture(b)
	runProjectEncodingBenchmarks(b, fixture)
}

func runProjectEncodingBenchmarks(b *testing.B, fixture projectBenchmarkFixture) {
	b.Helper()

	b.Run("protobuf/marshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var data []byte
		var err error
		for i := 0; i < b.N; i++ {
			data, err = proto.Marshal(fixture.proto)
			if err != nil {
				b.Fatal(err)
			}
		}

		benchmarkBytesSink = data
	})

	b.Run("tego/to_proto", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var project *shapespbv1.Project
		for i := 0; i < b.N; i++ {
			project = shapes.ProjectToProto(fixture.tego)
		}

		benchmarkProjectProtoSink = project
	})

	b.Run("tego/to_proto_then_marshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var data []byte
		var err error
		for i := 0; i < b.N; i++ {
			project := shapes.ProjectToProto(fixture.tego)
			data, err = proto.Marshal(project)
			if err != nil {
				b.Fatal(err)
			}
		}

		benchmarkBytesSink = data
	})

	b.Run("tego/json_marshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var data []byte
		var err error
		for i := 0; i < b.N; i++ {
			data, err = json.Marshal(fixture.tego)
			if err != nil {
				b.Fatal(err)
			}
		}

		benchmarkBytesSink = data
	})

	b.Run("tego/msgp_marshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var data []byte
		var err error
		for i := 0; i < b.N; i++ {
			data, err = fixture.tego.MarshalMsg(nil)
			if err != nil {
				b.Fatal(err)
			}
		}

		benchmarkBytesSink = data
	})
}

func BenchmarkProjectDecoding(b *testing.B) {
	fixture := newProjectBenchmarkFixture(b)
	runProjectDecodingBenchmarks(b, fixture)
}

func BenchmarkLargeProjectDecoding(b *testing.B) {
	fixture := newLargeProjectBenchmarkFixture(b)
	runProjectDecodingBenchmarks(b, fixture)
}

func runProjectDecodingBenchmarks(b *testing.B, fixture projectBenchmarkFixture) {
	b.Helper()

	b.Run("protobuf/unmarshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var project shapespbv1.Project
		for i := 0; i < b.N; i++ {
			project = shapespbv1.Project{}
			if err := proto.Unmarshal(fixture.protoBytes, &project); err != nil {
				b.Fatal(err)
			}
		}

		benchmarkProjectProtoSink = &project
	})

	b.Run("tego/from_proto", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var project shapes.Project
		for i := 0; i < b.N; i++ {
			project = shapes.ProjectFromProto(fixture.proto)
		}

		benchmarkProjectTegoSink = project
	})

	b.Run("tego/unmarshal_then_from_proto", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var tegoProject shapes.Project
		var protoProject shapespbv1.Project
		for i := 0; i < b.N; i++ {
			protoProject = shapespbv1.Project{}
			if err := proto.Unmarshal(fixture.protoBytes, &protoProject); err != nil {
				b.Fatal(err)
			}
			tegoProject = shapes.ProjectFromProto(&protoProject)
		}

		benchmarkProjectProtoSink = &protoProject
		benchmarkProjectTegoSink = tegoProject
	})

	b.Run("tego/json_unmarshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var project shapes.Project
		for i := 0; i < b.N; i++ {
			project = shapes.Project{}
			if err := json.Unmarshal(fixture.jsonBytes, &project); err != nil {
				b.Fatal(err)
			}
		}

		benchmarkProjectTegoSink = project
	})

	b.Run("tego/msgp_unmarshal", func(b *testing.B) {
		defer reportProjectFixtureMetrics(b, fixture)
		b.ReportAllocs()
		b.ResetTimer()

		var project shapes.Project
		for i := 0; i < b.N; i++ {
			project = shapes.Project{}
			leftover, err := project.UnmarshalMsg(fixture.msgpBytes)
			if err != nil {
				b.Fatal(err)
			}
			if len(leftover) > 0 {
				b.Fatalf("msgp unmarshal left %d trailing bytes", len(leftover))
			}
		}

		benchmarkProjectTegoSink = project
	})
}

func reportProjectFixtureMetrics(b *testing.B, fixture projectBenchmarkFixture) {
	b.Helper()

	b.ReportMetric(float64(len(fixture.protoBytes)), "proto_bytes")
	b.ReportMetric(float64(len(fixture.jsonBytes)), "json_bytes")
	b.ReportMetric(float64(len(fixture.msgpBytes)), "msgp_bytes")
}

func newProjectBenchmarkFixture(tb testing.TB) projectBenchmarkFixture {
	tb.Helper()

	return newProjectBenchmarkFixtureFromProject(tb, newBenchmarkProject())
}

func newLargeProjectBenchmarkFixture(tb testing.TB) projectBenchmarkFixture {
	tb.Helper()

	return newProjectBenchmarkFixtureFromProject(tb, newLargeBenchmarkProject(largeProjectBenchmarkScale))
}

func newProjectBenchmarkFixtureFromProject(tb testing.TB, tegoProject shapes.Project) projectBenchmarkFixture {
	tb.Helper()

	protoProject := shapes.ProjectToProto(tegoProject)

	protoBytes, err := proto.Marshal(protoProject)
	if err != nil {
		tb.Fatal(err)
	}

	var decodedProto shapespbv1.Project
	if err := proto.Unmarshal(protoBytes, &decodedProto); err != nil {
		tb.Fatal(err)
	}
	_ = shapes.ProjectFromProto(&decodedProto)

	jsonBytes, err := json.Marshal(tegoProject)
	if err != nil {
		tb.Fatal(err)
	}

	var decodedTego shapes.Project
	if err := json.Unmarshal(jsonBytes, &decodedTego); err != nil {
		tb.Fatal(err)
	}
	_ = shapes.ProjectToProto(decodedTego)

	msgpBytes, err := tegoProject.MarshalMsg(nil)
	if err != nil {
		tb.Fatal(err)
	}

	var decodedMsgp shapes.Project
	leftover, err := decodedMsgp.UnmarshalMsg(msgpBytes)
	if err != nil {
		tb.Fatal(err)
	}
	if len(leftover) > 0 {
		tb.Fatalf("msgp unmarshal left %d trailing bytes", len(leftover))
	}
	_ = shapes.ProjectToProto(decodedMsgp)

	return projectBenchmarkFixture{
		tego:       tegoProject,
		proto:      protoProject,
		protoBytes: protoBytes,
		jsonBytes:  jsonBytes,
		msgpBytes:  msgpBytes,
	}
}

func newBenchmarkProject() shapes.Project {
	owner := shapes.Person{
		ID:          "person_owner",
		DisplayName: "Avery Shah",
	}
	platformLead := shapes.Person{
		ID:          "person_platform",
		DisplayName: "Morgan Lee",
	}
	apiLead := shapes.Person{
		ID:          "person_api",
		DisplayName: "Riley Chen",
	}
	frontendLead := shapes.Person{
		ID:          "person_frontend",
		DisplayName: "Sam Rivera",
	}
	reviewerOne := shapes.Person{
		ID:          "person_reviewer_one",
		DisplayName: "Jordan Taylor",
	}
	reviewerTwo := shapes.Person{
		ID:          "person_reviewer_two",
		DisplayName: "Casey Patel",
	}
	previousOwner := shapes.Person{
		ID:          "person_previous_owner",
		DisplayName: "Jamie Brooks",
	}

	return shapes.Project{
		Slug: "tego-serialization-benchmarks",
		Members: map[string]shapes.Person{
			"api":      apiLead,
			"frontend": frontendLead,
			"platform": platformLead,
		},
		Owner: &owner,
		Reviewers: []shapes.Person{
			reviewerOne,
			reviewerTwo,
			platformLead,
		},
		Aliases: []string{
			"tego-bench",
			"serialization-bench",
			"mapping-bench",
		},
		LocalizedSlugs: map[string]string{
			"de-DE": "tego-serialisierung-benchmarks",
			"en-GB": "tego-serialization-benchmarks",
			"fr-FR": "benchmarks-serialisation-tego",
		},
		PreviousOwners: []*shapes.Person{
			&previousOwner,
			nil,
			&platformLead,
		},
		ContactsByRole: map[string]*shapes.Person{
			"api":      &apiLead,
			"frontend": &frontendLead,
			"owner":    &owner,
			"support":  nil,
		},
	}
}

func newLargeBenchmarkProject(scale int) shapes.Project {
	project := newBenchmarkProject()

	project.Slug = fmt.Sprintf("%s-%dx", project.Slug, scale)
	project.Members = make(map[string]shapes.Person, 3*scale)
	project.Reviewers = make([]shapes.Person, 0, 3*scale)
	project.Aliases = make([]string, 0, 3*scale)
	project.LocalizedSlugs = make(map[string]string, 3*scale)
	project.PreviousOwners = make([]*shapes.Person, 0, 3*scale)
	project.ContactsByRole = make(map[string]*shapes.Person, 4*scale)

	people := make([]shapes.Person, 0, 4*scale)
	for i := 0; i < 4*scale; i++ {
		people = append(people, shapes.Person{
			ID:          fmt.Sprintf("person_large_%03d", i),
			DisplayName: fmt.Sprintf("Large Benchmark Person %03d", i),
		})
	}
	if len(people) > 0 {
		project.Owner = &people[0]
	}

	for i := 0; i < 3*scale; i++ {
		person := people[i%len(people)]
		project.Members[fmt.Sprintf("team_%03d", i)] = person
		project.Reviewers = append(project.Reviewers, person)
		project.Aliases = append(project.Aliases, fmt.Sprintf("tego-bench-large-%03d", i))
		project.LocalizedSlugs[fmt.Sprintf("locale-%03d", i)] = fmt.Sprintf("tego-serialization-benchmarks-large-%03d", i)

		if i%5 == 0 {
			project.PreviousOwners = append(project.PreviousOwners, nil)
		} else {
			previousOwner := people[(i+1)%len(people)]
			project.PreviousOwners = append(project.PreviousOwners, &previousOwner)
		}
	}

	for i := 0; i < 4*scale; i++ {
		key := fmt.Sprintf("role_%03d", i)
		if i%7 == 0 {
			project.ContactsByRole[key] = nil
			continue
		}
		contact := people[i%len(people)]
		project.ContactsByRole[key] = &contact
	}

	return project
}
