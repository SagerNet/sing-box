package option

import "encoding/json"

type Listable[T any] []T

func (l *Listable[T]) MarshalJSON() ([]byte, error) {
	arrayList := []T(*l)
	if len(arrayList) == 1 {
		return json.Marshal(arrayList[0])
	}
	return json.Marshal(arrayList)
}

func (l *Listable[T]) UnmarshalJSON(bytes []byte) error {
	err := json.Unmarshal(bytes, (*[]T)(l))
	if err == nil {
		return nil
	}
	var singleItem T
	err = json.Unmarshal(bytes, &singleItem)
	if err != nil {
		return err
	}
	*l = []T{singleItem}
	return nil
}
