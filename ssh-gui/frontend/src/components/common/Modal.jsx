export default function Modal({
  open,
  children,
  onClose,
}) {

  if (!open) {
    return null;
  }

  return (

    <div
      className="modal-overlay"
      onClick={onClose}
    >

      <div
        className="modern-modal"
        onClick={(e) =>
          e.stopPropagation()
        }
      >

        {children}

      </div>

    </div>

  );

}