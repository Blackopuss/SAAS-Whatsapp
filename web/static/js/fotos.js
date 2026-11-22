// Reduce la foto en el celular (lado mayor 1600 px, WebP) antes de subirla, para ahorrar datos a la vendedora.
// El servidor vuelve a validar y recodificar todo. Sin JS, o si algo falla, el formulario sube el archivo tal cual.
const MAX = 1600;

async function shrink(file) {
  const bmp = await createImageBitmap(file, { imageOrientation: "from-image" });
  const scale = Math.min(1, MAX / Math.max(bmp.width, bmp.height));
  const canvas = document.createElement("canvas");
  canvas.width = Math.round(bmp.width * scale);
  canvas.height = Math.round(bmp.height * scale);
  canvas.getContext("2d").drawImage(bmp, 0, 0, canvas.width, canvas.height);
  bmp.close();
  const blob = await new Promise((ok) => canvas.toBlob(ok, "image/webp", 0.85));
  // Safari viejo no codifica WebP y devuelve PNG; si sale más pesado que el original, mandamos el original.
  return blob && blob.size < file.size ? blob : file;
}

for (const form of document.querySelectorAll("[data-foto-form]")) {
  form.addEventListener("submit", async (ev) => {
    const input = form.querySelector("input[type=file]");
    const file = input.files[0];
    if (!file || !("createImageBitmap" in window)) return;
    ev.preventDefault();
    const button = form.querySelector("button[type=submit]");
    const status = document.querySelector("[data-foto-estado]");
    button.disabled = true;
    button.textContent = "Subiendo…";
    status.replaceChildren(Object.assign(document.createElement("p"), { role: "status", textContent: "Preparando y subiendo la foto…" }));
    try {
      const data = new FormData(form);
      data.set("foto", await shrink(file), "foto.webp");
      const res = await fetch(form.action, { method: "POST", body: data });
      location.assign(res.url);
    } catch {
      form.submit(); // respaldo: subida normal sin reducir
    }
  });
}
