import { useId } from "react";

export function Input({ label, value, onChange, type = "text", placeholder, disabled, ...props }) {
  const id = useId();
  return <div className="form-group">
    <label className="form-label" htmlFor={id}>{label}</label>
    <input id={id} className="modern-input" type={type} value={value ?? ""} placeholder={placeholder}
      disabled={disabled} onChange={event => onChange(event.target.value)} {...props} />
  </div>;
}
export function Select({ label, value, onChange, children, disabled }) {
  const id = useId();
  return <div className="form-group">
    <label className="form-label" htmlFor={id}>{label}</label>
    <select id={id} className="modern-input" value={value ?? ""} disabled={disabled}
      onChange={event => onChange(event.target.value)}>{children}</select>
  </div>;
}
