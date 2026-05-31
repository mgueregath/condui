export default function Modal({
  open,
  children,
  onClose,
}) {

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay">

      <div className="modal">

        {children}

      </div>

    </div>
  );

}
