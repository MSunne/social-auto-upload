package store

const DigitalHumanCreditMillisScale int64 = CreditMillisScale

func DigitalHumanCreditsFromMillis(millis int64) float64 {
	return CreditsFromMillis(millis)
}

func DigitalHumanCreditsPtrFromMillis(millis *int64) *float64 {
	return CreditsPtrFromMillis(millis)
}

func DigitalHumanCreditsToMillis(value float64) (int64, error) {
	return CreditsToMillis(value)
}

func DigitalHumanRoundMillisToWholeCredits(millis int64) int64 {
	return RoundMillisToWholeCredits(millis)
}

func DigitalHumanCombineWalletBalanceMillis(wholeCredits int64, fractionalMillis int64) int64 {
	return CombineWalletBalanceMillis(wholeCredits, fractionalMillis)
}

func DigitalHumanSplitWalletBalanceMillis(totalMillis int64) (int64, int64) {
	return SplitWalletBalanceMillis(totalMillis)
}
