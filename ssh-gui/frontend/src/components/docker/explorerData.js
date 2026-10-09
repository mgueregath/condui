export const containerName = container => container.Name.replace(/^\//, "");
export const imageTags = image => image.RepoTags?.length ? image.RepoTags.join(", ") : "";
export const imageReferences = (containers, image) => containers.filter(container => container.Image === image.Id);

export function groupContainers(containers) {
 const groups = new Map();
 for (const container of containers) {
  const labels = container.Config.Labels || {};
  const type = labels["com.docker.stack.namespace"] ? "Swarm" : labels["com.docker.compose.project"] ? "Compose" : "";
  const project = labels["com.docker.stack.namespace"] || labels["com.docker.compose.project"] || "";
  const key = type + ":" + project;
  if (!groups.has(key)) groups.set(key, {key, type, project, containers: []});
  groups.get(key).containers.push(container);
 }
 return [...groups.values()].sort((a, b) => a.project.localeCompare(b.project));
}

export const volumeReferences = (containers, volume) => containers.filter(container =>
 (container.Mounts || []).some(mount => mount.Type === "volume" && mount.Name === volume.Name)
);
