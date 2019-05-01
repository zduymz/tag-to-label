package controller

import (
	"github.com/stretchr/testify/assert"
	"github.com/zduymz/tag-to-label/pkg/provider"
	"testing"
)

func TestFilterTag(t *testing.T) {
	input := map[string][]*provider.Tag{}
	input["key1"] = []*provider.Tag{
		{Key: "devops.apixio.com/tag1", Value: "value1"},
		{Key: "devops.apixio.com/tag2", Value: "value2"},
		{Key: "should-be-filtered", Value: "value3"},
	}
	input["key2"] = []*provider.Tag{
		{Key: "devops.apixio.com/tag3", Value: "value3"},
	}
	expect := map[string][]*provider.Tag{}
	expect["key1"] = []*provider.Tag{
		{Key: "devops.apixio.com/tag1", Value: "value1"},
		{Key: "devops.apixio.com/tag2", Value: "value2"},
	}
	expect["key2"] = []*provider.Tag{
		{Key: "devops.apixio.com/tag3", Value: "value3"},
	}
	output := FilterTag(input)
	assert.True(t, assert.ObjectsAreEqual(expect, output))
}

func TestTrimTag(t *testing.T) {
	input := []*provider.Tag{
		{Key: "devops.apixio.com/tag1", Value: "value1"},
		{Key: "devops.apixio.com/tag2", Value: "value2"},
		{Key: "should-be-filtered", Value: "value3"},
	}
	expect := map[string]string{
		"tag1": "value1",
		"tag2": "value2",
	}
	output := TrimTag(input)
	assert.True(t, assert.ObjectsAreEqual(expect, output))
}

func TestOuterRightJoin(t *testing.T) {
	left := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}
	right := map[string]string{
		"key3": "value3",
		"key1": "value11",
	}
	expect := map[string]string{
		"key1": "value11",
		"key3": "value3",
	}
	output, _ := OuterRightJoin(left, right)
	assert.True(t, assert.ObjectsAreEqual(expect, output))
}
