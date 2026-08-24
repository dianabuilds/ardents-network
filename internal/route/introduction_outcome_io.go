package route

import "io"

func WriteIntroductionSlotReady(writer io.Writer, input IntroductionSlotReady) error {
	raw, err := EncodeIntroductionSlotReady(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

func ReadIntroductionSlotReady(reader io.Reader) (IntroductionSlotReady, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return IntroductionSlotReady{}, err
	}
	return DecodeIntroductionSlotReady(raw)
}

func WriteIntroductionDeliveryResult(writer io.Writer, input IntroductionDeliveryResult) error {
	raw, err := EncodeIntroductionDeliveryResult(input)
	if err != nil {
		return err
	}
	return writeAll(writer, raw)
}

func ReadIntroductionDeliveryResult(reader io.Reader) (IntroductionDeliveryResult, error) {
	raw, err := readRouteRecord(reader)
	if err != nil {
		return IntroductionDeliveryResult{}, err
	}
	return DecodeIntroductionDeliveryResult(raw)
}
