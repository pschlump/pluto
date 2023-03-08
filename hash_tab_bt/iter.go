package hash_tab

func (tt *HashTab[T]) WalkFunc(Fx func(a *T)) {
	for i := 0; i < tt.length; i++ {
		if tt.buckets[i] != nil {
			tt.buckets[i].WalkFunc(Fx)
		}
	}
}
