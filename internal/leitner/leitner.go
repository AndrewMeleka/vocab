package leitner

import (
	"time"

	"github.com/andrewnageh/vocab/internal/store"
)

const MaxBox = 4

// Interval returns the duration before the next review for a given box.
// boxDays is read from config; if box overflows it pins to the last entry.
func Interval(box int, boxDays []int) time.Duration {
	if len(boxDays) == 0 {
		return 24 * time.Hour
	}
	if box < 0 {
		box = 0
	}
	if box >= len(boxDays) {
		box = len(boxDays) - 1
	}
	return time.Duration(boxDays[box]) * 24 * time.Hour
}

// Apply records a review result on a card and returns the updated card plus the
// ReviewEvent that should be persisted to the reviews table by the caller.
// Knew → box+1 (capped). Forgot → back to box 0. NextDueAt advances from now.
func Apply(c store.Card, result store.ReviewResult, now time.Time, boxDays []int) (store.Card, store.ReviewEvent) {
	ev := store.ReviewEvent{At: now, Result: result, Box: c.Box}
	switch result {
	case store.ResultKnew:
		if c.Box < MaxBox {
			c.Box++
		}
		c.CorrectStreak++
	case store.ResultForgot:
		c.Box = 0
		c.CorrectStreak = 0
		c.WrongCount++
	}
	t := now
	c.LastReviewedAt = &t
	c.NextDueAt = now.Add(Interval(c.Box, boxDays))
	return c, ev
}
