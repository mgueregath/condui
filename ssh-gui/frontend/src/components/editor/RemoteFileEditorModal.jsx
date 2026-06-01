import Editor from "@monaco-editor/react";

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


  return (

    <div className="remote-editor-backdrop">


      <div className="remote-editor-window">


        <div className="remote-editor-header">


          <div>

            <div className="remote-editor-title">

              {fileName}

              {modified && (
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

          <Editor

            height="100%"

            theme="vs-dark"

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
              }

            }}


            onChange={(v)=>
              onChange(
                v ?? ""
              )
            }

          />

        </div>



        <div className="remote-editor-footer">


          <button
            className="btn-secondary"
            onClick={onClose}
          >
            Close
          </button>


          <button

            className="btn-primary"

            disabled={!modified}

            onClick={onSave}

          >
            Save
          </button>


        </div>


      </div>


    </div>

  );
}