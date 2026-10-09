import { Call } from "@wailsio/runtime";

// Named calls keep these wrappers independent of generated binding IDs.
export const getSnapshot = session => Call.ByName("main.App.GetDockerExplorerSnapshot", session);
export const deleteImage = (session, image) => Call.ByName("main.App.DeleteDockerExplorerImage", session, image);
export const controlStack = (session, project, action) => Call.ByName("main.App.ControlDockerExplorerStack", session, project, action);
export const deleteVolume = (session, name) => Call.ByName("main.App.DeleteDockerExplorerVolume", session, name);
