package views

// Style es un estilo visual de la tienda. Todas usan el mismo marcado; cambia el tema de Astryx
// (frontend/src/themes/<id>), que Go pone en <html data-astryx-theme>.
type Style struct {
	ID, Name, Hint, Font string // Font: archivo en /static/fonts que se precarga
	Brand                string // color sugerido al elegir el estilo
}

var Styles = []Style{
	{"vitrina", "Vitrina", "Para cualquier giro: limpio y colorido.", "bricolage.woff2", "#c2185b"},
	{"boutique", "Boutique", "Ropa, accesorios y belleza: elegante, tipo revista.", "cormorant.woff2", "#5b2245"},
	{"barberia", "Barbería", "Barberías y productos para caballero: oscuro y con carácter.", "bigshoulders.woff2", "#b3202a"},
	{"carniceria", "Carnicería", "Carnes, pollerías y salchichonerías: letrero clásico de mostrador.", "alfaslab.woff2", "#8e1b1b"},
	{"abarrotes", "Abarrotes", "Tienditas y misceláneas: colores de rótulo pintado a mano.", "lilita.woff2", "#2f6b2f"},
	{"papeleria", "Papelería", "Papelerías y regalos: alegre, como cuaderno de cuadrícula.", "fredoka.woff2", "#1d4ed8"},
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

// scheme: barbería es el único estilo oscuro; va en <html> para que el tema pinte antes de JS.
func (s Style) scheme() string {
	if s.ID == "barberia" {
		return "dark"
	}
	return "light"
}
