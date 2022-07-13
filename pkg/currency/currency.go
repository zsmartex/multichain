package currency

type Currency struct {
	ID       string
	Subunits int32 // 8 -> 18
	Options  map[string]interface{}
}
