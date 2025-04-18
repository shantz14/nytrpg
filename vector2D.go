package main

type Vector2D struct {
	x, y float64
}

func (vec Vector2D) add(otherVec Vector2D) {
	vec.x += otherVec.x
	vec.y += otherVec.y
}

func (vec Vector2D) subtract(otherVec Vector2D) {
	vec.x -= otherVec.x
	vec.y -= otherVec.y
}

func (vec Vector2D) equals(otherVec Vector2D) bool {
	return (vec.x == otherVec.x && vec.y == otherVec.y)
}
