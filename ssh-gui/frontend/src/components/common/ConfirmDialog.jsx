export default function ConfirmDialog({
  open,
  title,
  onConfirm,
  onCancel,
}) {

  if (!open) {
    return null;
  }

  return (
    <div className="modal-overlay">

      <div className="modal">

        <h3>{title}</h3>

        <div
          style={{
            display:"flex",
            gap:"10px",
            marginTop:"20px",
          }}
        >

          <button
            onClick={onConfirm}
          >
            Confirmar
          </button>

          <button
            onClick={onCancel}
          >
            Cancelar
          </button>

        </div>

      </div>

    </div>
  );

}
