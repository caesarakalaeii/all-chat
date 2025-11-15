package normalizer

import "testing"

func TestValidateChannelID(t *testing.T) {
	valid := []string{"channel123", "UCxxxxxx", "my-channel", "user_name"}
	invalid := []string{"", "../bad", "bad?param", "name with space"}

	for _, id := range valid {
		if err := validateChannelID(id); err != nil {
			t.Fatalf("expected %q to be valid, got %v", id, err)
		}
	}

	for _, id := range invalid {
		if err := validateChannelID(id); err == nil {
			t.Fatalf("expected %q to be invalid", id)
		}
	}
}
