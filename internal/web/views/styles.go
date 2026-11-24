package views

// Style es un estilo visual de la tienda: su tema de Astryx (frontend/src/themes/<id>, que Go pone en
// <html data-astryx-theme>) y su portada y composición propias (shop.tsx y frontend/src/giros/<id>.css).
type Style struct {
	ID, Name, Hint, Font string // Font: archivo en /static/fonts que se precarga
	Brand                string // color sugerido al elegir el estilo
}

var Styles = []Style{
	{"vitrina", "Vitrina", "Para cualquier giro: buscador grande, colección destacada y categorías a la vista.", "bricolage.woff2", "#c8394f"},
	{"boutique", "Boutique", "Ropa y accesorios: portada tipo revista con foto grande y colecciones.", "cormorant.woff2", "#6d1f33"},
	{"barberia", "Barbería", "Productos para caballero: letrero oscuro, familias numeradas y fichas claras.", "bigshoulders.woff2", "#b4532a"},
	{"carniceria", "Carnicería", "Carnes y salchichonería: pizarra de precios del día y venta por kilo o pieza.", "alfaslab.woff2", "#8c1c1c"},
	{"abarrotes", "Abarrotes", "Tienditas: buscador primero, un pasillo por categoría y compra rápida.", "lilita.woff2", "#2f6b3a"},
	{"papeleria", "Papelería", "Papelerías y regalos: hoja cuadriculada y catálogo en carpetas de colores.", "fredoka.woff2", "#2f6fd6"},
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

// scheme: todos los estilos son claros; barbería usa bandas oscuras dentro de la página, no fondo negro.
func (s Style) scheme() string { return "light" }
