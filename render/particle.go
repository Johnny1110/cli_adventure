// render/particle.go — Ambient particle system for atmospheric effects.
//
// THEORY — Particles as visual storytelling:
// Particles are tiny, short-lived visual elements that make a game world
// feel alive. A forest without falling leaves feels static; a cave without
// floating dust feels dead. Each particle is dead simple: position, velocity,
// lifetime, color. Spawn a bunch, update them each tick, remove when expired.
//
// The trick at Game Boy resolution is restraint — too many particles become
// noise. We use 8-15 particles at a time, each just 1-2 pixels. The effect
// is subtle but the difference between "a grid of tiles" and "a living world"
// is enormous.
//
// Each area type gets its own particle behavior:
//   Forest: leaves drift down and sideways, gentle sway
//   Cave:   dust motes float upward slowly, faint glow
//   Lair:   embers rise fast, bright orange, short-lived
package render

import (
	"image/color"
	"math"
	"math/rand"

	"github.com/hajimehoshi/ebiten/v2"
)

// Particle is a single visual particle.
type Particle struct {
	X, Y     float64
	VX, VY   float64
	Life     int // ticks remaining
	MaxLife  int
	Color    color.RGBA
	Size     int // 1 or 2 pixels
}

// ParticleSystem manages a collection of ambient particles.
type ParticleSystem struct {
	particles []*Particle
	areaType  string
	width     int // area pixel width
	height    int // area pixel height
	tick      int
}

// NewParticleSystem creates a particle system for the given area type.
func NewParticleSystem(areaType string, areaW, areaH int) *ParticleSystem {
	return &ParticleSystem{
		areaType: areaType,
		width:    areaW * TileSize,
		height:   areaH * TileSize,
	}
}

// Update spawns new particles and advances existing ones.
func (ps *ParticleSystem) Update() {
	ps.tick++

	// Spawn new particles periodically
	switch ps.areaType {
	case "forest":
		if ps.tick%12 == 0 && len(ps.particles) < 15 {
			ps.spawnLeaf()
		}
	case "cave":
		if ps.tick%20 == 0 && len(ps.particles) < 10 {
			ps.spawnDust()
		}
	case "lair":
		if ps.tick%6 == 0 && len(ps.particles) < 12 {
			ps.spawnEmber()
		}
	}

	// Update existing particles
	alive := ps.particles[:0]
	for _, p := range ps.particles {
		p.X += p.VX
		p.Y += p.VY
		p.Life--

		// Forest leaves: gentle sway
		if ps.areaType == "forest" {
			p.VX = 0.3 * math.Sin(float64(ps.tick+int(p.Y))*0.05)
		}

		if p.Life > 0 {
			alive = append(alive, p)
		}
	}
	ps.particles = alive
}

// Draw renders all particles relative to the camera.
func (ps *ParticleSystem) Draw(screen *ebiten.Image, camX, camY int) {
	for _, p := range ps.particles {
		sx := int(p.X) - camX
		sy := int(p.Y) - camY

		// Only draw if on screen
		if sx < -2 || sx > 162 || sy < -2 || sy > 146 {
			continue
		}

		// Fade out near end of life
		alpha := p.Color.A
		if p.Life < 20 {
			alpha = uint8(float64(alpha) * float64(p.Life) / 20.0)
		}

		clr := color.RGBA{R: p.Color.R, G: p.Color.G, B: p.Color.B, A: alpha}
		screen.Set(sx, sy, clr)
		if p.Size >= 2 {
			screen.Set(sx+1, sy, clr)
			screen.Set(sx, sy+1, clr)
		}
	}
}

func (ps *ParticleSystem) spawnLeaf() {
	// Leaves: green/yellow/orange, drift down from top
	colors := []color.RGBA{
		{R: 120, G: 180, B: 80, A: 200},  // green leaf
		{R: 180, G: 160, B: 60, A: 200},  // yellow leaf
		{R: 200, G: 120, B: 60, A: 180},  // orange leaf
		{R: 100, G: 160, B: 70, A: 160},  // dark green
	}
	ps.particles = append(ps.particles, &Particle{
		X:       float64(rand.Intn(ps.width)),
		Y:       float64(rand.Intn(ps.height / 3)),
		VX:      0.2,
		VY:      0.3 + rand.Float64()*0.3,
		Life:    120 + rand.Intn(80),
		MaxLife: 200,
		Color:   colors[rand.Intn(len(colors))],
		Size:    1 + rand.Intn(2),
	})
}

func (ps *ParticleSystem) spawnDust() {
	// Dust: faint white/blue, float upward slowly
	colors := []color.RGBA{
		{R: 160, G: 160, B: 180, A: 100},  // gray dust
		{R: 140, G: 160, B: 200, A: 80},   // blue-ish dust
		{R: 180, G: 180, B: 200, A: 120},  // bright dust
	}
	ps.particles = append(ps.particles, &Particle{
		X:       float64(rand.Intn(ps.width)),
		Y:       float64(ps.height/2 + rand.Intn(ps.height/2)),
		VX:      (rand.Float64() - 0.5) * 0.15,
		VY:      -0.1 - rand.Float64()*0.15,
		Life:    150 + rand.Intn(100),
		MaxLife: 250,
		Color:   colors[rand.Intn(len(colors))],
		Size:    1,
	})
}

func (ps *ParticleSystem) spawnEmber() {
	// Embers: bright orange/red, rise fast from bottom
	colors := []color.RGBA{
		{R: 255, G: 140, B: 40, A: 220},   // bright orange
		{R: 255, G: 80, B: 30, A: 200},    // red-orange
		{R: 255, G: 200, B: 60, A: 180},   // yellow
		{R: 255, G: 100, B: 50, A: 240},   // hot
	}
	ps.particles = append(ps.particles, &Particle{
		X:       float64(rand.Intn(ps.width)),
		Y:       float64(ps.height/2 + rand.Intn(ps.height/2)),
		VX:      (rand.Float64() - 0.5) * 0.4,
		VY:      -0.5 - rand.Float64()*0.5,
		Life:    40 + rand.Intn(40),
		MaxLife: 80,
		Color:   colors[rand.Intn(len(colors))],
		Size:    1,
	})
}
