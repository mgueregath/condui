export default function RemoteFileNode({
  item,
  onOpen,
}) {

  return (

    <div
      onDoubleClick={() =>
        onOpen(item)
      }
      style={{
        padding: "6px 10px",
        cursor: "pointer",
      }}
    >

      {
        item.isDirectory
          ? "📁"
          : "📄"
      }

      {" "}

      {item.name}

    </div>

  );

}
