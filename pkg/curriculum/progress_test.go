package curriculum

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestZeroProgressReadsAsNotStarted(t *testing.T) {
	var p Progress
	if got := p.LessonState("anything"); got != StateNotStarted {
		t.Fatalf("state = %q, want %q from the zero value", got, StateNotStarted)
	}
	var nilp *Progress
	if got := nilp.LessonState("anything"); got != StateNotStarted {
		t.Fatalf("state = %q from a nil *Progress, want %q", got, StateNotStarted)
	}
}

// An unrecognised state must be refused, not stored: the gate reads this
// vocabulary, so a value outside it is a row nothing can act on.
func TestUnknownLessonStateIsRejected(t *testing.T) {
	p := NewProgress()
	err := p.MarkLesson("les-1", "nearly-done")
	if !errors.Is(err, ErrUnknownLessonState) {
		t.Fatalf("err = %v, want ErrUnknownLessonState", err)
	}
	if _, stored := p.Lessons["les-1"]; stored {
		t.Fatal("the rejected state was stored anyway")
	}
	for _, want := range []string{"not-started", "in-progress", "complete"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error does not name the accepted value %q: %v", want, err)
		}
	}
}

func TestEmptyLessonIDIsRejected(t *testing.T) {
	if err := NewProgress().MarkLesson("", StateComplete); err == nil {
		t.Fatal("an empty lesson id was accepted")
	}
}

func TestProgressRoundTripsThroughJSON(t *testing.T) {
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	p.RecordAttempt(Attempt{AssessmentID: "asm-1", At: at, Percent: 80, Passed: true, Determinate: true})

	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	var back Progress
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if back.LessonState("les-1") != StateComplete {
		t.Fatal("lesson state did not survive the round trip")
	}
	if len(back.AttemptsFor("asm-1")) != 1 {
		t.Fatal("the attempt did not survive the round trip")
	}
}

// The persistence claim in the README is that a consumer can store Progress
// with nothing but encoding/json and a byte sink. This asserts it end to end
// against a real file, so the claim is measured rather than described.
func TestProgressPersistsToAFileAndBack(t *testing.T) {
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	_ = p.MarkLesson("les-2", StateInProgress)

	path := filepath.Join(t.TempDir(), "progress.json")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var back Progress
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	av, err := AvailabilityOf(area1(), &back)
	if err != nil {
		t.Fatal(err)
	}
	if av.Available || len(av.MissingLessons) != 1 {
		t.Fatalf("restored progress gates differently: %+v", av)
	}
}

// An attempt this package could not fully grade must never be reported as a
// best result: its percent is a lower bound, and offering it as an achievement
// would be a claim about work nothing measured.
func TestBestAttemptIgnoresIndeterminateAttempts(t *testing.T) {
	p := NewProgress()
	p.RecordAttempt(Attempt{AssessmentID: "a", Percent: 95, Determinate: false})
	if _, ok := p.BestAttempt("a"); ok {
		t.Fatal("an indeterminate attempt was returned as the best result")
	}
	p.RecordAttempt(Attempt{AssessmentID: "a", Percent: 61, Determinate: true, Passed: true})
	best, ok := p.BestAttempt("a")
	if !ok || best.Percent != 61 {
		t.Fatalf("best = %+v, ok = %v; want the 61%% determinate attempt", best, ok)
	}
}

func TestCloneDoesNotShareState(t *testing.T) {
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	p.RecordAttempt(Attempt{AssessmentID: "a", At: time.Now()})

	c := p.Clone()
	_ = c.MarkLesson("les-1", StateInProgress)
	c.RecordAttempt(Attempt{AssessmentID: "a"})

	if p.LessonState("les-1") != StateComplete {
		t.Fatal("mutating the clone changed the original's lesson state")
	}
	if len(p.AttemptsFor("a")) != 1 {
		t.Fatal("mutating the clone changed the original's attempts")
	}
}

func TestCompletionTallies(t *testing.T) {
	a := area1()
	p := NewProgress()
	_ = p.MarkLesson("les-1", StateComplete)
	_ = p.MarkLesson("les-2", StateInProgress)
	c := Completion(a, p)
	if c.TotalLessons != 2 || c.CompleteLessons != 1 || c.InProgressLesson != 1 || c.Percent != 50 {
		t.Fatalf("completion = %+v", c)
	}
}

func TestCompletionOfAnEmptyAreaIsZeroNotOneHundred(t *testing.T) {
	c := Completion(Area{ID: "empty"}, NewProgress())
	if c.Percent != 0 {
		t.Fatalf("percent = %d for an area with no lessons; an empty set is not fully complete", c.Percent)
	}
}
