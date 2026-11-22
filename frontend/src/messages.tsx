// "Mensajes": plantillas de WhatsApp para cada etapa de la compra. La vendedora las copia y llena lo que está
// entre {{ }}. Nada inventa descuentos, existencias ni urgencia: esos datos los pone ella o no van.
import { useState } from "react";
import { Copy, Check } from "lucide-react";
import { Button } from "@astryxdesign/core/Button";
import { Card } from "@astryxdesign/core/Card";
import { Heading } from "@astryxdesign/core/Heading";
import { Text } from "@astryxdesign/core/Text";
import { Token } from "@astryxdesign/core/Token";
import { HStack, VStack } from "@astryxdesign/core/Stack";

type Tpl = { id: string; stage: string; name: string; goal: string; when: string; cta: string; metric: string; text: string };

function templates(store: string, url: string, payment: boolean): Tpl[] {
  const pay = payment ? "Pago por transferencia; te paso los datos al confirmar." : "Pago: {{formas_de_pago}}.";
  return [
    {
      id: "bienvenida", stage: "1 · Bienvenida", name: "Primer mensaje", goal: "Que encuentre rápido lo que busca.",
      when: "Cuando alguien te escribe por primera vez sin pedido.", cta: "Que te diga qué busca.",
      metric: "De cada 10 primeros mensajes, cuántos terminan en pedido en 7 días.",
      text: `¡Hola, {{nombre}}! Gracias por escribir a ${store}. Aquí ves todo lo disponible con precio y tallas: ${url}\n\nSi me dices qué buscas (talla, color o para qué ocasión), te paso las opciones que tengo.`,
    },
    {
      id: "recomendacion", stage: "2 · Recomendación", name: "Te recomiendo esto", goal: "Ayudar a elegir con base en lo que pidió.",
      when: "Solo después de que te dijo qué busca.", cta: "Apartar el producto.",
      metric: "Recomendaciones que se vuelven pedido.",
      text: `{{nombre}}, por lo que me contaste ({{lo_que_busca}}), te recomiendo:\n\n• {{producto}}: {{precio}}\n  ${url}p/{{id_producto}}\n\nLo tengo en {{tallas_disponibles}}. ¿Te lo aparto?`,
    },
    {
      id: "dudas", stage: "3 · Dudas", name: "Respuesta antes de comprar", goal: "Quitar la duda con datos claros.",
      when: "Cuando pregunta por un producto, el pago o la entrega.", cta: "Confirmar si lo quiere.",
      metric: "Tiempo en responder y preguntas que terminan en pedido.",
      text: `Claro, {{nombre}}. Sobre {{producto}}: {{respuesta}}.\n\n${pay}\nEntrega: {{forma_y_tiempo_de_entrega}}.\n\n¿Quieres que te lo aparte?`,
    },
    {
      id: "pendiente", stage: "4 · Pedido pendiente", name: "Seguimiento de una sola vez", goal: "Retomar una compra que quedó a medias, sin presionar.",
      when: "Solo si ella te escribió en las últimas 24 horas y no confirmó. Una vez, no más.", cta: "Responder sí para pasarle el total.",
      metric: "Pedidos pendientes que se confirman.",
      text: `Hola, {{nombre}}. Te escribo una sola vez por {{producto}}: sigue disponible en {{talla}}. Si aún lo quieres, respóndeme "sí" y te paso el total. Si ya no, no te preocupes.`,
    },
    {
      id: "confirmacion", stage: "5 · Confirmación", name: "Pedido confirmado", goal: "Que sepa exactamente qué compró, cuánto y cuándo llega.",
      when: "En cuanto revisas existencias de un pedido.", cta: "Pagar o confirmar la entrega.",
      metric: "Pedidos confirmados que se pagan.",
      text: `¡Gracias, {{nombre}}! Confirmo tu pedido:\n\n{{resumen}}\nTotal: {{total}}\n\n${pay}\nEntrega: {{fecha_entrega}} por {{metodo_entrega}}.\n\nTe aviso cuando salga.`,
    },
    {
      id: "envio", stage: "5 · Seguimiento", name: "Tu pedido va en camino", goal: "Evitar la duda de ¿y mi pedido?",
      when: "Cuando el pedido sale o está listo para recoger.", cta: "Escribir si hay algún problema.",
      metric: "Preguntas de ¿dónde está mi pedido? por cada 10 pedidos.",
      text: `{{nombre}}, tu pedido ya va en camino. {{guia_u_hora_de_entrega}}\n\nSi algo no llega bien, escríbeme aquí y lo resolvemos.`,
    },
    {
      id: "recompra", stage: "6 · Recompra", name: "Llegó algo para ti", goal: "Avisar de algo nuevo que sí le sirve.",
      when: "Solo a quien ya compró y aceptó recibir avisos; con una novedad real.", cta: "Apartar el producto nuevo.",
      metric: "Clientas que vuelven a comprar en 60 días.",
      text: `Hola, {{nombre}}. Llegó {{producto_nuevo}} en {{tallas}}, parecido a lo que te llevaste ({{compra_anterior}}): ${url}p/{{id_producto}}\n\nSi te interesa, respóndeme y te lo aparto.\n\nSi prefieres no recibir estos avisos, responde NO y ya no te escribo.`,
    },
    {
      id: "resena", stage: "7 · Reseña", name: "¿Cómo te quedó?", goal: "Saber si quedó contenta y, si quiere, compartir su opinión.",
      when: "3 a 5 días después de la entrega.", cta: "Contar cómo le fue (y una foto, si quiere).",
      metric: "Respuestas recibidas y problemas resueltos.",
      text: `Hola, {{nombre}}. ¿Cómo te quedó {{producto}}?\n\nSi algo no salió bien, dime y lo arreglamos. Y si te gustó, ¿me dejas compartir tu comentario con otras clientas? Solo si estás de acuerdo.`,
    },
    {
      id: "inactivas", stage: "8 · Clientas inactivas", name: "Una razón para volver", goal: "Reactivar con una novedad útil, no con presión.",
      when: "Solo con una novedad real (tallas nuevas, nueva zona de entrega). Máximo una vez cada 60 días.", cta: "Ver la tienda.",
      metric: "Clientas inactivas que vuelven a escribir o comprar.",
      text: `Hola, {{nombre}}. Te cuento que {{novedad_real}}. Aquí está todo: ${url}\n\nSi ya no quieres recibir mensajes de ${store}, responde NO.`,
    },
  ];
}

const TESTS = [
  { name: "Primer mensaje con o sin pregunta", how: "La mitad de las veces manda la bienvenida con la pregunta final; la otra mitad, solo la liga. Compara cuántas terminan en pedido." },
  { name: "Recomendar 1 o 3 productos", how: "Alterna una sola recomendación con tres opciones. Mide cuál se convierte más en pedido sin más preguntas." },
  { name: "Reseña con o sin foto", how: "Pide solo el comentario o comentario y foto. Mide cuántas responden; quédate con la que no baje las respuestas." },
];

// Resalta {{variables}} para que se vea qué falta llenar.
function Body({ text }: { text: string }) {
  return (
    <Text as="p" className="tpl-text">
      {text.split(/(\{\{[a-z_]+\}\})/).map((part, i) =>
        part.startsWith("{{") ? <mark key={i} className="tpl-var">{part}</mark> : part,
      )}
    </Text>
  );
}

function CopyButton({ text, name }: { text: string; name: string }) {
  const [done, setDone] = useState(false);
  return (
    <Button size="sm" variant={done ? "secondary" : "primary"} label={done ? `Copiado: ${name}` : `Copiar ${name}`}
      icon={done ? <Check aria-hidden size={16} /> : <Copy aria-hidden size={16} />}
      onClick={() => navigator.clipboard.writeText(text).then(() => { setDone(true); setTimeout(() => setDone(false), 2000); })}>
      {done ? "Copiado" : "Copiar"}
    </Button>
  );
}

export function MessagesBody({ store, shopURL, payment }: { store: string; shopURL: string; payment: string }) {
  const list = templates(store, shopURL, payment !== "");
  return (
    <VStack gap={5}>
      <Text color="secondary">
        Copia el mensaje, cambia lo que está marcado entre {"{{ }}"} y mándalo desde tu WhatsApp. No pongas descuentos, existencias ni fechas que no sean reales.
      </Text>
      {list.map((t, i) => (
        <Card key={t.id} padding={5} className="rise" style={{ "--i": i } as React.CSSProperties}>
          <VStack gap={3}>
            <HStack gap={3} hAlign="between" vAlign="center" wrap="wrap">
              <VStack gap={1}>
                <Token size="sm" label={t.stage} />
                <Heading level={2}>{t.name}</Heading>
              </VStack>
              <CopyButton text={t.text} name={t.name} />
            </HStack>
            <Body text={t.text} />
            <dl className="tpl-meta">
              <dt>Para qué</dt><dd>{t.goal}</dd>
              <dt>Cuándo</dt><dd>{t.when}</dd>
              <dt>Qué esperas</dt><dd>{t.cta}</dd>
              <dt>Cómo saber si funciona</dt><dd>{t.metric}</dd>
            </dl>
          </VStack>
        </Card>
      ))}
      <Card padding={5}>
        <VStack gap={3}>
          <Heading level={2}>Tres pruebas sencillas</Heading>
          <Text color="secondary">Cambia una sola cosa a la vez durante dos semanas y anota en una libreta los resultados.</Text>
          <ol className="tpl-tests">
            {TESTS.map((t) => <li key={t.name}><Text weight="semibold">{t.name}.</Text> <Text color="secondary">{t.how}</Text></li>)}
          </ol>
        </VStack>
      </Card>
    </VStack>
  );
}
