// Campos de Astryx para formularios nativos (method="post"): cada uno guarda su valor y lo envía con htmlName.
// Así los handlers de Go, la protección CSRF y las redirecciones siguen igual que con templ.
import { useState, type ReactNode } from "react";
import { TextInput, type TextInputProps } from "@astryxdesign/core/TextInput";
import { TextArea, type TextAreaProps } from "@astryxdesign/core/TextArea";
import { CheckboxInput } from "@astryxdesign/core/CheckboxInput";
import { Selector } from "@astryxdesign/core/Selector";
import { Banner } from "@astryxdesign/core/Banner";
import { Button, type ButtonProps } from "@astryxdesign/core/Button";
import { VStack } from "@astryxdesign/core/Stack";

type Named = { name: string; initial?: string };

export function Txt({ name, initial = "", ...p }: Named & Omit<TextInputProps, "value" | "onChange" | "htmlName">) {
  const [v, setV] = useState(initial);
  return <TextInput {...p} htmlName={name} value={v} onChange={setV} />;
}

export function Area({ name, initial = "", ...p }: Named & Omit<TextAreaProps, "value" | "onChange" | "htmlName">) {
  const [v, setV] = useState(initial);
  return <TextArea {...p} htmlName={name} value={v} onChange={setV} />;
}

export function Check({ name, label, initial = false, description, value }: { name: string; label: string; initial?: boolean; description?: string; value?: string }) {
  const [on, setOn] = useState(initial);
  // CheckboxInput envía "on"; si el servidor espera otro valor (soldout=talla|color) va en un hidden.
  if (value === undefined) return <CheckboxInput label={label} description={description} htmlName={name} value={on} onChange={setOn} />;
  return (
    <>
      <CheckboxInput label={label} description={description} value={on} onChange={setOn} />
      {on && <input type="hidden" name={name} value={value} />}
    </>
  );
}

export function Pick({ name, label, options, initial = "", description, placeholder }: { name: string; label: string; options: { value: string; label: string }[]; initial?: string; description?: string; placeholder?: string }) {
  const [v, setV] = useState(initial);
  return (
    <>
      <Selector label={label} description={description} placeholder={placeholder} options={options} value={v} onChange={(x) => setV(x ?? "")} presentation="adaptive" width="100%" />
      <input type="hidden" name={name} value={v} />
    </>
  );
}

// Msg: error (role=alert) y confirmación (role=status) que manda el servidor tras un POST.
export function Msg({ error, ok, warn }: { error?: string; ok?: string; warn?: ReactNode }) {
  if (!error && !ok && !warn) return null;
  return (
    <VStack gap={2}>
      {error && <Banner status="error" title={error} role="alert" />}
      {ok && <Banner status="success" title={ok} role="status" />}
      {warn && <Banner status="warning" title={warn} />}
    </VStack>
  );
}

// PostButton: un botón que hace POST a action (marcar, borrar, mover…). fields van como hidden.
export function PostButton({ action, fields, ...b }: { action: string; fields?: Record<string, string> } & Omit<ButtonProps, "type">) {
  return (
    <form method="post" action={action} className="inline-form">
      {fields && Object.entries(fields).map(([k, v]) => <input key={k} type="hidden" name={k} value={v} />)}
      <Button {...b} type="submit" />
    </form>
  );
}

// Form: formulario nativo con la separación vertical del sistema.
export function Form({ action, children, multipart, gap = 4 }: { action: string; children: ReactNode; multipart?: boolean; gap?: 2 | 3 | 4 | 5 }) {
  return (
    <form method="post" action={action} noValidate encType={multipart ? "multipart/form-data" : undefined}>
      <VStack gap={gap}>{children}</VStack>
    </form>
  );
}
