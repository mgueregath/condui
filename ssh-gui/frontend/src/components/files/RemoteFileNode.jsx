import { FaFolder, FaFolderOpen, FaFile } from "react-icons/fa";


export default function RemoteFileNode({
 item,
 onOpen,
 onOpenFile,
 onContextMenu,
}){

 const isDir=item.isDirectory;


 return(
  <div
   className="file-node"

   onDoubleClick={()=>{

    if(isDir)
      onOpen(item);
    else
      onOpenFile(item);

   }}

   onContextMenu={(e)=>{

    e.preventDefault();

    onContextMenu?.(
      item,
      e.clientX,
      e.clientY
    );

   }}
  >

   <div className="file-node-name">

    <span>
     {isDir? <FaFolder /> : <FaFile /> }
    </span>

    <span>
     {item.name}
    </span>

   </div>

  </div>
 );

}