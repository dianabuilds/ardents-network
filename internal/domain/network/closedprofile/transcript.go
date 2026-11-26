package closedprofile

func signingTranscript(body []byte) []byte {
	return append([]byte("ardents-closed-profile-v3\x00"), body...)
}
