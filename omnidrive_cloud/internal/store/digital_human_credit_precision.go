package store

import (
	"fmt"
	"math"
)

const DigitalHumanCreditMillisScale int64 = 1000

func DigitalHumanCreditsFromMillis(millis int64) float64 {
	return float64(millis) / float64(DigitalHumanCreditMillisScale)
}

func DigitalHumanCreditsPtrFromMillis(millis *int64) *float64 {
	if millis == nil {
		return nil
	}
	value := DigitalHumanCreditsFromMillis(*millis)
	return &value
}

func DigitalHumanCreditsToMillis(value float64) (int64, error) {
	if value < 0 {
		return 0, fmt.Errorf("credits must be greater than or equal to 0")
	}

	scaled := value * float64(DigitalHumanCreditMillisScale)
	rounded := math.Round(scaled)
	if math.Abs(scaled-rounded) > 1e-6 {
		return 0, fmt.Errorf("credits supports up to 3 decimal places")
	}
	return int64(rounded), nil
}

func DigitalHumanRoundMillisToWholeCredits(millis int64) int64 {
	if millis <= 0 {
		return 0
	}
	return int64(math.Round(float64(millis) / float64(DigitalHumanCreditMillisScale)))
}

func DigitalHumanCombineWalletBalanceMillis(wholeCredits int64, fractionalMillis int64) int64 {
	return wholeCredits*DigitalHumanCreditMillisScale + fractionalMillis
}

func DigitalHumanSplitWalletBalanceMillis(totalMillis int64) (int64, int64) {
	wholeCredits := totalMillis / DigitalHumanCreditMillisScale
	fractionalMillis := totalMillis % DigitalHumanCreditMillisScale
	if fractionalMillis < 0 {
		fractionalMillis += DigitalHumanCreditMillisScale
		wholeCredits--
	}
	return wholeCredits, fractionalMillis
}
