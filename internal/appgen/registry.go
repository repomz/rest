package appgen

func DefaultRegistry() []Feature {
	return []Feature{
		MongoFeature{},
		SQLFeature{},
	}
}
