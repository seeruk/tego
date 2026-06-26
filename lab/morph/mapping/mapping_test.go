package mapping

import (
	"testing"

	"github.com/seeruk/tego/lab/morph/tego"
	"github.com/stretchr/testify/assert"
)

func TestShapeMappingLab(t *testing.T) {
	t.Run("round trips nested shaped people", func(t *testing.T) {
		source := tego.Fixture{
			ID: "fixture-1",
			People: []*[]tego.Person{
				nil,
				people(
					tego.Person{FirstName: "Ada", LastName: "Lovelace"},
					tego.Person{FirstName: "Grace", LastName: "Hopper"},
				),
				people(),
			},
		}

		proto := MapTegoFixtureFromMorphpbv1Fixture(source)
		got := MapMorphpbv1FixtureToTegoFixture(proto)

		assert.Equal(t, source, got)
	})
}

func people(values ...tego.Person) *[]tego.Person {
	out := make([]tego.Person, len(values))
	copy(out, values)
	return &out
}
