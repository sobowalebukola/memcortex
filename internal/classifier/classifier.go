package classifier

import (
	"os"
	"slices"
	"strconv"
	"strings"
)

// EventType categorizes the kind of meaningful event a message represents.
type EventType string

const (
	Preference  EventType = "preference"
	Goal        EventType = "goal"
	Correction  EventType = "correction"
	Profile     EventType = "profile"
	Achievement EventType = "achievement"
	General     EventType = "general"
	Trivial     EventType = "trivial"
)

// Result is the output of classifying a single message.
type Result struct {
	ShouldSave bool
	Type       EventType
	Weight     float64
	TTLDays    int // 0 = no expiry
}

// Classifier scores incoming messages to decide whether they are worth persisting.
// It uses a multi-signal approach: positive pattern rules boost weight, negative
// signals (questions, negations, third-person references) apply penalties.
type Classifier struct {
	MinWeight float64
}

type rule struct {
	eventType  EventType
	baseWeight float64
	ttlDays    int
	patterns   []string
}

// rules are checked in order; the highest-baseWeight match wins.
var rules = []rule{
	{
		eventType:  Correction,
		baseWeight: 0.95,
		ttlDays:    0, // corrections never expire
		patterns: []string{
			"actually my ", "actually i ", "i meant ", "correction:",
			"i'm actually ", "my role is actually", "i was wrong",
			"that's not right", "let me correct", "to clarify,",
			"i should clarify", "to be precise",
		},
	},
	{
		eventType:  Profile,
		baseWeight: 0.90,
		ttlDays:    0,
		patterns: []string{
			"my name is", "i work at", "i'm a ", "i am a ",
			"i live in", "my email is", "my job is", "i work as",
			"call me ", "i'm based in", "i'm from ",
			"my phone", "my address", "my birthday",
			"i specialize in", "my title is", "my team is",
		},
	},
	{
		eventType:  Preference,
		baseWeight: 0.90,
		ttlDays:    0,
		patterns: []string{
			"i prefer", "i like ", "my favorite", "i enjoy ",
			"i love ", "i hate ", "i dislike", "i don't like",
			"i always use", "i never use", "my go-to ",
			"i'm a fan of", "i tend to use", "i usually ",
			"i'd rather ", "my preferred", "i opt for",
			"i'm not a fan of",
		},
	},
	{
		eventType:  Goal,
		baseWeight: 0.85,
		ttlDays:    90,
		patterns: []string{
			"remind me to", "i want to learn", "i need to finish",
			"i plan to", "my goal is", "i'm going to start",
			"i'm working on", "i'm trying to", "i aim to",
			"i intend to", "by next ", "my deadline",
			"i'm studying", "i'm building", "i'm learning",
			"i want to build", "i need to complete",
		},
	},
	{
		eventType:  Achievement,
		baseWeight: 0.80,
		ttlDays:    365,
		patterns: []string{
			"i completed", "i finished", "i achieved",
			"i got promoted", "i graduated", "i passed ",
			"i built ", "i shipped ", "i launched",
			"i earned", "i published", "i released",
			"i solved", "i won ", "i was promoted",
			"i got certified", "i was hired",
		},
	},
}

func New() *Classifier {
	minWeight := 0.3
	if v, err := strconv.ParseFloat(os.Getenv("MIN_EVENT_WEIGHT"), 64); err == nil && v > 0 {
		minWeight = v
	}
	return &Classifier{MinWeight: minWeight}
}

func (c *Classifier) Classify(text string) Result {
	lower := strings.ToLower(strings.TrimSpace(text))

	// Trivial messages are never worth saving.
	if len(lower) < 10 || isTrivial(lower) {
		return Result{Type: Trivial, Weight: 0.0}
	}

	// Find the best matching rule. When multiple rules match with equal
	// weight, prefer the longest pattern (most specific match).
	var bestRule *rule
	var bestLen int
	for i := range rules {
		for _, p := range rules[i].patterns {
			if strings.Contains(lower, p) {
				if bestRule == nil ||
					rules[i].baseWeight > bestRule.baseWeight ||
					(rules[i].baseWeight == bestRule.baseWeight && len(p) > bestLen) {
					bestRule = &rules[i]
					bestLen = len(p)
				}
			}
		}
	}
	best := bestRule

	// Compute penalty from negative signals.
	pen := penalty(lower)

	if best != nil {
		w := clamp(best.baseWeight-pen, 0, 1)
		return Result{
			ShouldSave: w >= c.MinWeight,
			Type:       best.eventType,
			Weight:     w,
			TTLDays:    best.ttlDays,
		}
	}

	// No pattern matched — score as general content.
	w := clamp(generalWeight(lower)-pen, 0, 1)
	return Result{
		ShouldSave: w >= c.MinWeight,
		Type:       General,
		Weight:     w,
		TTLDays:    30,
	}
}

// penalty reduces weight based on signals that the message is not a
// meaningful first-person statement worth persisting.
func penalty(text string) float64 {
	p := 0.0

	if isQuestion(text) {
		p += 0.25
	}
	if hasNegation(text) {
		p += 0.15
	}
	if isThirdPerson(text) {
		p += 0.35
	}

	return p
}

// generalWeight scores unmatched messages by word count as a rough proxy
// for substantive content.
func generalWeight(text string) float64 {
	words := len(strings.Fields(text))
	switch {
	case words > 25:
		return 0.45
	case words > 15:
		return 0.35
	case words > 8:
		return 0.25
	default:
		return 0.15
	}
}

func isQuestion(text string) bool {
	if strings.Contains(text, "?") {
		return true
	}
	starters := []string{
		"what ", "who ", "where ", "when ", "why ", "how ",
		"do you", "can you", "could you", "would you",
		"is there", "are there", "does ", "did ",
	}
	for _, s := range starters {
		if strings.HasPrefix(text, s) {
			return true
		}
	}
	return false
}

func hasNegation(text string) bool {
	// Pad with spaces so we match whole words only.
	// Without this, "nothing" would falsely match "not ".
	padded := " " + text + " "
	negations := []string{
		" don't ", " doesn't ", " didn't ", " not ", " never ",
		" no longer ", " can't ", " won't ", " wouldn't ",
	}
	for _, n := range negations {
		if strings.Contains(padded, n) {
			return true
		}
	}
	return false
}

// isThirdPerson returns true when the message talks about someone else
// rather than the user themselves. Uses space-padded matching to avoid
// substring false positives (e.g. "the" matching "he").
func isThirdPerson(text string) bool {
	padded := " " + text + " "

	firstPerson := []string{" i ", " i'm ", " i've ", " i'd ", " my ", " me "}
	thirdPerson := []string{" he ", " she ", " they ", " his ", " her ", " their "}

	hasFirst := false
	for _, fp := range firstPerson {
		if strings.Contains(padded, fp) {
			hasFirst = true
			break
		}
	}

	hasThird := false
	for _, tp := range thirdPerson {
		if strings.Contains(padded, tp) {
			hasThird = true
			break
		}
	}

	// Only penalize when message references others but not the user.
	return hasThird && !hasFirst
}

var trivialPhrases = []string{
	// Acknowledgements
	"ok", "okay", "sure", "yes", "no", "yeah", "nah", "yep", "nope",
	"k", "kk", "yup", "ya", "ye",
	// Gratitude
	"thanks", "thank you", "thx", "ty", "np", "no problem",
	// Greetings
	"hi", "hello", "hey", "yo", "sup", "howdy",
	// Farewells
	"bye", "goodbye", "see you", "later", "cya",
	// Reactions
	"lol", "haha", "heh", "lmao", "rofl",
	// Agreement / filler
	"got it", "cool", "nice", "great", "good", "fine", "alright",
	"sounds good", "makes sense", "fair enough", "right", "true",
	"same", "agreed", "exactly", "indeed", "absolutely",
	// Hesitation
	"hmm", "hm", "um", "uh", "idk", "idc",
	// Bare question words
	"what", "why", "how", "when", "where",
}

func isTrivial(text string) bool {
	cleaned := strings.TrimRight(text, ".!?,")
	return slices.Contains(trivialPhrases, cleaned)
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
