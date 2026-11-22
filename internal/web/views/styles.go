package views

// Style es un estilo visual de la tienda. Todas usan el mismo marcado; cambia vitrina.css vía data-estilo.
type Style struct {
	ID, Name, Hint, Font string // Font: archivo en /static/fonts que se precarga
}

var Styles = []Style{
	{"vitrina", "Vitrina", "Para cualquier giro: limpio y colorido.", "bricolage.woff2"},
	{"boutique", "Boutique", "Ropa, accesorios y belleza: elegante, tipo revista.", "cormorant.woff2"},
	{"barberia", "Barbería", "Barberías y productos para caballero: oscuro y con carácter.", "bigshoulders.woff2"},
	{"carniceria", "Carnicería", "Carnes, pollerías y salchichonerías: letrero clásico de mostrador.", "alfaslab.woff2"},
	{"abarrotes", "Abarrotes", "Tienditas y misceláneas: colores de rótulo pintado a mano.", "lilita.woff2"},
	{"papeleria", "Papelería", "Papelerías y regalos: alegre, como cuaderno de cuadrícula.", "fredoka.woff2"},
}

// StyleByID devuelve el estilo o Vitrina si no existe.
func StyleByID(id string) Style {
	for _, s := range Styles {
		if s.ID == id {
			return s
		}
	}
	return Styles[0]
}

// ValidStyle indica si id es un estilo conocido.
func ValidStyle(id string) bool { return StyleByID(id).ID == id }
