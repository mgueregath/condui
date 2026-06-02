import Editor from "@monaco-editor/react";


const getExtension = (path = "") => {
  return path
    .split(".")
    .pop()
    ?.toLowerCase();
};


const getLanguage = (path) => {

  const ext = getExtension(path);


  const map = {

    js: "javascript",
    jsx: "javascript",

    ts: "typescript",
    tsx: "typescript",

    json: "json",

    html: "html",
    css: "css",

    go: "go",

    py: "python",

    java: "java",

    sh: "shell",

    yaml: "yaml",
    yml: "yaml",

    md: "markdown",

    sql: "sql",

    xml: "xml",

    env: "plaintext",

    log: "plaintext",
    txt: "plaintext",

  };


  return map[ext] || "plaintext";

};



const isImage = (path) => {

  const ext = getExtension(path);


  return [
    "png",
    "jpg",
    "jpeg",
    "gif",
    "webp",
    "bmp",
    "svg",
  ].includes(ext);

};




export default function RemoteFileEditorModal({
  open,
  path,
  content,
  modified,
  onChange,
  onClose,
  onSave,
}) {


  if (!open) return null;


  const fileName =
    path?.split("/").pop();


  const image =
    isImage(path);


  const language =
    getLanguage(path);



  return (

    <div className="remote-editor-backdrop">

      <div className="remote-editor-window">


        <div className="remote-editor-header">

          <div>

            <div className="remote-editor-title">

              {fileName}


              {modified && !image && (

                <span className="editor-modified">
                  ●
                </span>

              )}


            </div>


            <div className="remote-editor-path">

              {path}

            </div>

          </div>


          <button
            className="remote-editor-close"
            onClick={onClose}
          >
            ×
          </button>


        </div>



        <div className="remote-editor-body">


          {image ? (


            <div className="remote-image-viewer">


              <img

                src={`data:image/*;base64,${content}`}

                alt={fileName}

              />


            </div>


          ) : (


            <Editor

              height="100%"

              theme="vs-dark"

              language={language}

              value={content}


              options={{

                minimap:{
                  enabled:false,
                },

                fontSize:14,

                automaticLayout:true,

                scrollBeyondLastLine:false,

                wordWrap:"on",

                padding:{
                  top:12,
                },

              }}


              onChange={(v)=>
                onChange(
                  v ?? ""
                )
              }


            />


          )}


        </div>



        <div className="remote-editor-footer">


          <button
            className="btn-secondary"
            onClick={onClose}
          >
            Close
          </button>



          {!image && (

            <button

              className="btn-primary"

              disabled={!modified}

              onClick={onSave}

            >

              Save

            </button>

          )}


        </div>


      </div>

    </div>

  );

}