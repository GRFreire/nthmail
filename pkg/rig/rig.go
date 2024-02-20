package rig

import (
	"fmt"
	"math/rand"
)

func GenerateRandomInboxName() string {
    animal := animals[rand.Intn(len(animals))]
    color := colors[rand.Intn(len(colors))]
    adjective := adjectives[rand.Intn(len(adjectives))]

    return fmt.Sprintf("%s-%s-%s", adjective, color, animal)
}
