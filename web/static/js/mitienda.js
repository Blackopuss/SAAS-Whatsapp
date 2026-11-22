// Vista previa en vivo: al cambiar nombre o color, recarga el <iframe> con los valores sin guardar.
const form = document.querySelector("[data-preview-form]");
const frame = document.querySelector("[data-preview]");
if (form && frame) {
  let t;
  const update = () => {
    clearTimeout(t);
    t = setTimeout(() => {
      const q = new URLSearchParams({ nombre: form.elements.name.value, color: form.elements.color.value });
      frame.src = "/panel/tienda/vista?" + q;
    }, 300);
  };
  form.elements.name.addEventListener("input", update);
  form.elements.color.addEventListener("input", update);
}
