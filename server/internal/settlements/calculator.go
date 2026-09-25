package settlements

import (
	"errors"
	"fmt"
	"math"
	"sort"

	"github.com/Fyy10/settled/server/internal/identifier"
	"github.com/Fyy10/settled/server/internal/money"
)

var (
	ErrInvalidDebtEntry   = errors.New("invalid settlement debt entry")
	ErrArithmeticOverflow = errors.New("settlement arithmetic overflow")
)

type PairwiseCalculator struct{}

func (PairwiseCalculator) Calculate(
	entries []DebtEntry,
) ([]Transfer, error) {
	directed := make(map[directedPair]int64)
	for index, entry := range entries {
		if err := validateDebtEntry(entry); err != nil {
			return nil, fmt.Errorf("debt entry %d: %w", index, err)
		}
		pair := directedPair{
			fromUserID: entry.FromUserID,
			toUserID:   entry.ToUserID,
		}
		total, err := money.Add(directed[pair], entry.AmountCents)
		if err != nil {
			return nil, fmt.Errorf(
				"%w: aggregate debt entry %d",
				ErrArithmeticOverflow,
				index,
			)
		}
		directed[pair] = total
	}

	unordered := make(map[unorderedPair]struct{}, len(directed))
	for pair := range directed {
		unordered[canonicalPair(pair.fromUserID, pair.toUserID)] = struct{}{}
	}
	transfers := make([]Transfer, 0, len(unordered))
	for pair := range unordered {
		fromAToB := directed[directedPair{
			fromUserID: pair.userAID,
			toUserID:   pair.userBID,
		}]
		fromBToA := directed[directedPair{
			fromUserID: pair.userBID,
			toUserID:   pair.userAID,
		}]
		net, err := money.Add(fromAToB, -fromBToA)
		if err != nil || net == math.MinInt64 {
			return nil, fmt.Errorf(
				"%w: offset users %s and %s",
				ErrArithmeticOverflow,
				pair.userAID,
				pair.userBID,
			)
		}
		if net == 0 {
			continue
		}
		transfer := Transfer{
			FromUserID:  pair.userAID,
			ToUserID:    pair.userBID,
			AmountCents: net,
			Currency:    money.CurrencyUSD,
		}
		if net < 0 {
			transfer.FromUserID = pair.userBID
			transfer.ToUserID = pair.userAID
			transfer.AmountCents = -net
		}
		transfers = append(transfers, transfer)
	}
	sort.Slice(transfers, func(left, right int) bool {
		if transfers[left].AmountCents != transfers[right].AmountCents {
			return transfers[left].AmountCents > transfers[right].AmountCents
		}
		if transfers[left].FromUserID != transfers[right].FromUserID {
			return transfers[left].FromUserID < transfers[right].FromUserID
		}
		return transfers[left].ToUserID < transfers[right].ToUserID
	})
	return transfers, nil
}

type directedPair struct {
	fromUserID string
	toUserID   string
}

type unorderedPair struct {
	userAID string
	userBID string
}

func validateDebtEntry(entry DebtEntry) error {
	fromUserID, err := identifier.ParseUUID(entry.FromUserID)
	if err != nil || fromUserID != entry.FromUserID {
		return fmt.Errorf("%w: from-user ID must be a canonical UUID", ErrInvalidDebtEntry)
	}
	toUserID, err := identifier.ParseUUID(entry.ToUserID)
	if err != nil || toUserID != entry.ToUserID {
		return fmt.Errorf("%w: to-user ID must be a canonical UUID", ErrInvalidDebtEntry)
	}
	if entry.FromUserID == entry.ToUserID {
		return fmt.Errorf("%w: users must differ", ErrInvalidDebtEntry)
	}
	if entry.AmountCents <= 0 {
		return fmt.Errorf("%w: amount must be positive", ErrInvalidDebtEntry)
	}
	if entry.Currency != money.CurrencyUSD {
		return fmt.Errorf("%w: currency must be USD", ErrInvalidDebtEntry)
	}
	return nil
}

func canonicalPair(left string, right string) unorderedPair {
	if left < right {
		return unorderedPair{userAID: left, userBID: right}
	}
	return unorderedPair{userAID: right, userBID: left}
}
