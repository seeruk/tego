package tego

import (
	"encoding/json"
	"testing"

	"github.com/seeruk/tego/examples/shapes/shapes"
	"github.com/seeruk/tego/examples/shapes/shapespbv1"
	"google.golang.org/protobuf/proto"
)

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
}

func BenchmarkProjectEncoding(b *testing.B) {
	fixture := newProjectBenchmarkFixture(b)

	b.Run("protobuf/marshal", func(b *testing.B) {
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
		b.ReportAllocs()
		b.ResetTimer()

		var project *shapespbv1.Project
		for i := 0; i < b.N; i++ {
			project = shapes.ProjectToProto(fixture.tego)
		}

		benchmarkProjectProtoSink = project
	})

	b.Run("tego/to_proto_then_marshal", func(b *testing.B) {
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
}

func BenchmarkProjectDecoding(b *testing.B) {
	fixture := newProjectBenchmarkFixture(b)

	b.Run("protobuf/unmarshal", func(b *testing.B) {
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
		b.ReportAllocs()
		b.ResetTimer()

		var project shapes.Project
		for i := 0; i < b.N; i++ {
			project = shapes.ProjectFromProto(fixture.proto)
		}

		benchmarkProjectTegoSink = project
	})

	b.Run("tego/unmarshal_then_from_proto", func(b *testing.B) {
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
}

func newProjectBenchmarkFixture(tb testing.TB) projectBenchmarkFixture {
	tb.Helper()

	tegoProject := newBenchmarkProject()
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

	return projectBenchmarkFixture{
		tego:       tegoProject,
		proto:      protoProject,
		protoBytes: protoBytes,
		jsonBytes:  jsonBytes,
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
