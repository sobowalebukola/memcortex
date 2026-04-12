package classifier

import (
	"os"
	"testing"
)

func newTestClassifier() *Classifier {
	os.Setenv("MIN_EVENT_WEIGHT", "0.3")
	return New()
}

// --- Trivial messages should always be skipped ---

func TestClassify_Trivial(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"ok", "OK", "Ok.", "okay", "sure", "yes", "no",
		"yeah", "nah", "yep", "nope",
		"thanks", "thanks!", "thank you", "thx", "ty",
		"hi", "hello", "hey", "yo",
		"bye", "goodbye", "see you", "later",
		"lol", "haha", "heh", "lmao",
		"got it", "cool", "nice", "great", "good", "fine",
		"sounds good", "makes sense", "fair enough",
		"hmm", "um", "idk",
		"what", "why", "how",
		// Under 10 chars, always trivial
		"short", "nah ok", "k",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if r.ShouldSave {
			t.Errorf("Classify(%q): expected skip, got save (type=%s, weight=%.2f)", input, r.Type, r.Weight)
		}
		if r.Type != Trivial {
			t.Errorf("Classify(%q): expected type=trivial, got %s", input, r.Type)
		}
	}
}

// --- Each event type should be correctly identified ---

func TestClassify_Preference(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"I prefer dark mode for all my editors",
		"I like using Vim for Go development",
		"My favorite language is Rust",
		"I always use tabs instead of spaces",
		"I'd rather use PostgreSQL than MySQL",
		"My preferred framework is Next.js",
		"I'm a fan of functional programming",
		"I tend to use Linux for development",
		"I'm not a fan of microservices",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if !r.ShouldSave {
			t.Errorf("Classify(%q): expected save, got skip (weight=%.2f)", input, r.Weight)
		}
		if r.Type != Preference {
			t.Errorf("Classify(%q): expected type=preference, got %s", input, r.Type)
		}
		if r.TTLDays != 0 {
			t.Errorf("Classify(%q): preferences should not expire, got TTL=%d", input, r.TTLDays)
		}
	}
}

func TestClassify_Profile(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"My name is Alex and I work at Google",
		"I'm a senior backend engineer",
		"I live in San Francisco",
		"I work as a data scientist at Meta",
		"Call me Alex",
		"I'm based in Berlin",
		"I specialize in distributed systems",
		"My title is Staff Engineer",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if !r.ShouldSave {
			t.Errorf("Classify(%q): expected save, got skip (weight=%.2f)", input, r.Weight)
		}
		if r.Type != Profile {
			t.Errorf("Classify(%q): expected type=profile, got %s", input, r.Type)
		}
		if r.TTLDays != 0 {
			t.Errorf("Classify(%q): profile info should not expire, got TTL=%d", input, r.TTLDays)
		}
	}
}

func TestClassify_Correction(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"Actually my role is backend engineer, not frontend",
		"I meant Python, not JavaScript",
		"I'm actually a contractor, not full-time",
		"That's not right, my team is platform engineering",
		"To clarify, I work on the payments service",
		"I was wrong about the deadline, it's next Friday",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if !r.ShouldSave {
			t.Errorf("Classify(%q): expected save, got skip (weight=%.2f)", input, r.Weight)
		}
		if r.Type != Correction {
			t.Errorf("Classify(%q): expected type=correction, got %s", input, r.Type)
		}
	}
}

func TestClassify_Goal(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"Remind me to study Rust this month",
		"I want to learn Kubernetes by next quarter",
		"I plan to migrate our services to gRPC",
		"I'm working on a side project in Go",
		"I'm trying to get my AWS certification",
		"I'm building a memory layer for LLMs",
		"I'm learning Japanese on the side",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if !r.ShouldSave {
			t.Errorf("Classify(%q): expected save, got skip (weight=%.2f)", input, r.Weight)
		}
		if r.Type != Goal {
			t.Errorf("Classify(%q): expected type=goal, got %s", input, r.Type)
		}
		if r.TTLDays != 90 {
			t.Errorf("Classify(%q): goals should have TTL=90, got %d", input, r.TTLDays)
		}
	}
}

func TestClassify_Achievement(t *testing.T) {
	c := newTestClassifier()
	cases := []string{
		"I shipped the new payment service last week",
		"I got promoted to Staff Engineer",
		"I graduated from MIT with a CS degree",
		"I launched my first open source project",
		"I earned my AWS Solutions Architect certification",
		"I was hired at Stripe last month",
	}
	for _, input := range cases {
		r := c.Classify(input)
		if !r.ShouldSave {
			t.Errorf("Classify(%q): expected save, got skip (weight=%.2f)", input, r.Weight)
		}
		if r.Type != Achievement {
			t.Errorf("Classify(%q): expected type=achievement, got %s", input, r.Type)
		}
		if r.TTLDays != 365 {
			t.Errorf("Classify(%q): achievements should have TTL=365, got %d", input, r.TTLDays)
		}
	}
}

// --- Negative signals should reduce weight ---

func TestClassify_QuestionPenalty(t *testing.T) {
	c := newTestClassifier()

	// A direct preference statement should have full weight.
	statement := c.Classify("I prefer dark mode for all my code editors")
	// The same content as a question should be penalized.
	question := c.Classify("Do you know what I prefer for my editors?")

	if question.Weight >= statement.Weight {
		t.Errorf("Question (weight=%.2f) should be penalized below statement (weight=%.2f)",
			question.Weight, statement.Weight)
	}
}

func TestClassify_ThirdPersonPenalty(t *testing.T) {
	c := newTestClassifier()

	// Third-person without first-person should be heavily penalized.
	r := c.Classify("He works at Google as a senior engineer")
	if r.ShouldSave {
		t.Errorf("Third-person message should be skipped, got save (type=%s, weight=%.2f)", r.Type, r.Weight)
	}

	// Mixed first+third person should NOT be penalized (user talking about themselves and others).
	r = c.Classify("I work at Google and he works at Meta")
	if !r.ShouldSave {
		t.Errorf("Mixed first+third person should be saved, got skip (weight=%.2f)", r.Weight)
	}
}

func TestClassify_NegationPenalty(t *testing.T) {
	c := newTestClassifier()

	positive := c.Classify("I prefer using dark mode in all editors")
	negated := c.Classify("I don't prefer using dark mode in any editors")

	if negated.Weight >= positive.Weight {
		t.Errorf("Negated (weight=%.2f) should be penalized below positive (weight=%.2f)",
			negated.Weight, positive.Weight)
	}
}

// --- General messages: length-based scoring ---

func TestClassify_GeneralShort_Skipped(t *testing.T) {
	c := newTestClassifier()
	r := c.Classify("the weather is nice today")
	if r.ShouldSave {
		t.Errorf("Short general message should be skipped, got save (weight=%.2f)", r.Weight)
	}
	if r.Type != General {
		t.Errorf("Expected type=general, got %s", r.Type)
	}
}

func TestClassify_GeneralLong_Saved(t *testing.T) {
	c := newTestClassifier()
	long := "We had a detailed architecture review today and decided to move the payment " +
		"processing pipeline from a monolith to a set of microservices using gRPC for " +
		"inter-service communication and Kafka for event streaming"
	r := c.Classify(long)
	if !r.ShouldSave {
		t.Errorf("Long substantive message should be saved, got skip (weight=%.2f)", r.Weight)
	}
	if r.TTLDays != 30 {
		t.Errorf("General messages should have TTL=30, got %d", r.TTLDays)
	}
}

// --- Boundary: highest-weight rule wins when multiple match ---

func TestClassify_HighestWeightWins(t *testing.T) {
	c := newTestClassifier()
	// Contains both correction pattern ("actually my") and profile pattern ("my name is").
	// Correction has higher baseWeight (0.95) so it should win.
	r := c.Classify("Actually my name is not Bob, it's Alex")
	if r.Type != Correction {
		t.Errorf("When correction and profile both match, correction (higher weight) should win, got %s", r.Type)
	}
}

// --- MinWeight threshold ---

func TestClassify_CustomMinWeight(t *testing.T) {
	os.Setenv("MIN_EVENT_WEIGHT", "0.99")
	defer os.Setenv("MIN_EVENT_WEIGHT", "0.3")

	c := New()
	// Even a strong signal shouldn't pass a very high threshold.
	r := c.Classify("Actually my role is backend engineer")
	// Correction base weight is 0.95, which is below 0.99.
	if r.ShouldSave {
		t.Errorf("With MinWeight=0.99, correction (weight=%.2f) should be skipped", r.Weight)
	}
}

// --- Edge cases ---

func TestClassify_EmptyString(t *testing.T) {
	c := newTestClassifier()
	r := c.Classify("")
	if r.ShouldSave {
		t.Errorf("Empty string should be skipped")
	}
}

func TestClassify_WhitespaceOnly(t *testing.T) {
	c := newTestClassifier()
	r := c.Classify("   \t  \n  ")
	if r.ShouldSave {
		t.Errorf("Whitespace-only should be skipped")
	}
}

func TestClassify_CaseInsensitive(t *testing.T) {
	c := newTestClassifier()
	r := c.Classify("MY NAME IS ALEX AND I WORK AT GOOGLE")
	if !r.ShouldSave {
		t.Errorf("Classification should be case-insensitive, got skip (weight=%.2f)", r.Weight)
	}
	if r.Type != Profile {
		t.Errorf("Expected type=profile, got %s", r.Type)
	}
}
