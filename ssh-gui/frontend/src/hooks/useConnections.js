import {
  useEffect,
  useState,
} from "react";

import {
  GetFolders,
  GetConnections,
} from "../../bindings/ssh-gui/app";

export function useConnections() {

  const [folders,
    setFolders] =
    useState([]);

  const [connections,
    setConnections] =
    useState([]);

  const load =
    async () => {

      const f =
        await GetFolders();

      const c =
        await GetConnections();

      setFolders(f || []);
      setConnections(c || []);

    };

  useEffect(() => {
    load();
  }, []);

  return {
    folders,
    connections,
    reload: load,
  };
}
