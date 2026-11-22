// Vista previa en vivo: al cambiar nombre, color o estilo, recarga el <iframe> con los valores sin guardar.
const form = document.querySelector("[data-preview-form]");
const frame = document.querySelector("[data-preview]");
if (form && frame) {
  let t;
  const update = () => {
    clearTimeout(t);
    t = setTimeout(() => {
      const q = new URLSearchParams({ nombre: form.elements.name.value, color: form.elements.color.value, estilo: form.elements.style.value });
      frame.src = "/panel/tienda/vista?" + q;
    }, 300);
  };
  form.elements.name.addEventListener("input", update);
  form.elements.color.addEventListener("input", update);
  for (const r of form.elements.style) {
    r.addEventListener("change", () => {
      form.elements.color.value = r.dataset.color; // color sugerido del estilo
      update();
    });
  }
}
