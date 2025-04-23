package main

type Vector2D struct {
	X float64 `msgpack:"x"`
	Y float64 `msgpack:"y"`
}

func (vec Vector2D) add(otherVec Vector2D) {
	vec.X += otherVec.X
	vec.Y += otherVec.Y
}

func (vec Vector2D) subtract(otherVec Vector2D) {
	vec.X -= otherVec.X
	vec.Y -= otherVec.Y
}

func (vec Vector2D) equals(otherVec Vector2D) bool {
	return (vec.X == otherVec.X && vec.Y == otherVec.Y)
}
