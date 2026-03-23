package workflow

import (
	"reflect"
	"testing"
)

func TestNormalizeSkillTopics(t *testing.T) {
	got := normalizeSkillTopics([]string{
		" #科技资讯 ",
		"科技资讯",
		"",
		"#新品开箱",
		"  新品开箱  ",
		"玩具测评",
	})

	want := []string{"科技资讯", "新品开箱", "玩具测评"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeSkillTopics() = %#v, want %#v", got, want)
	}
}
