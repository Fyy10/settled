package expenses

// SplitMode identifies the requested split calculation.
type SplitMode string

const (
	SplitModeEqual      SplitMode = "equal"
	SplitModeExact      SplitMode = "exact"
	SplitModePercentage SplitMode = "percentage"
)

// SplitInput contains exactly one split variant selected by Mode.
type SplitInput struct {
	Mode               SplitMode
	ParticipantUserIDs []string
	Splits             []ExactSplitInput
	PercentageSplits   []PercentageSplitInput
}

// ExactSplitInput assigns an exact cent amount to one participant.
type ExactSplitInput struct {
	UserID      string
	AmountCents int64
}

// PercentageSplitInput assigns positive basis points to one participant.
type PercentageSplitInput struct {
	UserID                string
	PercentageBasisPoints int64
}

// Split is the exact positive-cent persistence representation.
type Split struct {
	UserID      string
	AmountCents int64
}
