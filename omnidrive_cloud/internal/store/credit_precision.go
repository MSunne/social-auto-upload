package store

import (
	"fmt"
	"math"
)

const CreditMillisScale int64 = 1000

func CreditsFromMillis(millis int64) float64 {
	return float64(millis) / float64(CreditMillisScale)
}

func CreditsPtrFromMillis(millis *int64) *float64 {
	if millis == nil {
		return nil
	}
	value := CreditsFromMillis(*millis)
	return &value
}

func CreditsToMillis(value float64) (int64, error) {
	if value < 0 {
		return 0, fmt.Errorf("credits must be greater than or equal to 0")
	}

	scaled := value * float64(CreditMillisScale)
	rounded := math.Round(scaled)
	if math.Abs(scaled-rounded) > 1e-6 {
		return 0, fmt.Errorf("credits supports up to 3 decimal places")
	}
	return int64(rounded), nil
}

func RoundMillisToWholeCredits(millis int64) int64 {
	if millis <= 0 {
		return 0
	}
	return int64(math.Round(float64(millis) / float64(CreditMillisScale)))
}

func CombineWalletBalanceMillis(wholeCredits int64, fractionalMillis int64) int64 {
	return wholeCredits*CreditMillisScale + fractionalMillis
}

func SplitWalletBalanceMillis(totalMillis int64) (int64, int64) {
	wholeCredits := totalMillis / CreditMillisScale
	fractionalMillis := totalMillis % CreditMillisScale
	if fractionalMillis < 0 {
		fractionalMillis += CreditMillisScale
		wholeCredits--
	}
	return wholeCredits, fractionalMillis
}
