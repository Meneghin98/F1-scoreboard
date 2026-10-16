package telemetry

// CarMotion contiene posizione e dinamica di un'auto.
type CarMotion struct {
	WorldPosition   Vector3 // metri
	WorldVelocity   Vector3 // metri/s
	WorldForwardDir Vector3 // direzione normalizzata (già convertita da int16)
	WorldRightDir   Vector3

	// Forze G: in float nel formato 2025. Nel 2026 il gioco le invia "quantizzate"
	// in int16 senza documentare la scala, quindi restano grezze in GForceQuantised.
	GForceLateral, GForceLongitudinal, GForceVertical float32
	GForceQuantised                                   [3]int16 // solo 2026: laterale, longitudinale, verticale

	Yaw, Pitch, Roll float32 // radianti
}

// Motion (pacchetto 0) contiene il movimento di tutte le auto.
type Motion struct {
	Header
	Cars []CarMotion
}

func decodeMotion(h Header, r *reader, cars int) *Motion {
	p := &Motion{Header: h, Cars: make([]CarMotion, cars)}
	for i := range p.Cars {
		c := &p.Cars[i]
		c.WorldPosition = r.vec3()
		c.WorldVelocity = r.vec3()
		c.WorldForwardDir = r.normalized()
		c.WorldRightDir = r.normalized()
		if r.wide {
			c.GForceQuantised = [3]int16{r.i16(), r.i16(), r.i16()}
		} else {
			c.GForceLateral, c.GForceLongitudinal, c.GForceVertical = r.f32(), r.f32(), r.f32()
		}
		c.Yaw, c.Pitch, c.Roll = r.f32(), r.f32(), r.f32()
	}
	return p
}

// normalized legge un vettore normalizzato codificato in tre int16.
func (r *reader) normalized() Vector3 {
	const scale = 32767.0
	return Vector3{float32(r.i16()) / scale, float32(r.i16()) / scale, float32(r.i16()) / scale}
}

// MotionEx (pacchetto 13) contiene dati di movimento aggiuntivi dell'auto del giocatore.
// Gli array per ruota sono nell'ordine: posteriore sinistra, posteriore destra,
// anteriore sinistra, anteriore destra.
type MotionEx struct {
	Header
	SuspensionPosition     [4]float32
	SuspensionVelocity     [4]float32
	SuspensionAcceleration [4]float32
	WheelSpeed             [4]float32
	WheelSlipRatio         [4]float32
	WheelSlipAngle         [4]float32
	WheelLatForce          [4]float32
	WheelLongForce         [4]float32
	HeightOfCOGAboveGround float32
	LocalVelocity          Vector3
	AngularVelocity        Vector3
	AngularAcceleration    Vector3
	FrontWheelsAngle       float32 // radianti
	WheelVertForce         [4]float32
	FrontAeroHeight        float32
	RearAeroHeight         float32
	FrontRollAngle         float32
	RearRollAngle          float32
	ChassisYaw             float32
	ChassisPitch           float32
	WheelCamber            [4]float32
	WheelCamberGain        [4]float32
}

func decodeMotionEx(h Header, r *reader) *MotionEx {
	return &MotionEx{
		Header:                 h,
		SuspensionPosition:     r.wheels(),
		SuspensionVelocity:     r.wheels(),
		SuspensionAcceleration: r.wheels(),
		WheelSpeed:             r.wheels(),
		WheelSlipRatio:         r.wheels(),
		WheelSlipAngle:         r.wheels(),
		WheelLatForce:          r.wheels(),
		WheelLongForce:         r.wheels(),
		HeightOfCOGAboveGround: r.f32(),
		LocalVelocity:          r.vec3(),
		AngularVelocity:        r.vec3(),
		AngularAcceleration:    r.vec3(),
		FrontWheelsAngle:       r.f32(),
		WheelVertForce:         r.wheels(),
		FrontAeroHeight:        r.f32(),
		RearAeroHeight:         r.f32(),
		FrontRollAngle:         r.f32(),
		RearRollAngle:          r.f32(),
		ChassisYaw:             r.f32(),
		ChassisPitch:           r.f32(),
		WheelCamber:            r.wheels(),
		WheelCamberGain:        r.wheels(),
	}
}
