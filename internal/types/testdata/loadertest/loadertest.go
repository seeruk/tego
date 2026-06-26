package loadertest

type Example struct {
	Value string
}

func Convert(value string) Example {
	return Example{Value: value}
}

func (Example) ValueReceiver() string {
	return ""
}

func (*Example) PointerReceiver() string {
	return ""
}
