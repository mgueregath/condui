export namespace main {
	
	export class SystemStats {
	    cpuPercent: number;
	    memUsedGB: number;
	    memFreeGB: number;
	    memTotalGB: number;
	    diskUsedGB: number;
	    diskFreeGB: number;
	    diskTotalGB: number;
	    uptimeSecs: number;
	    netRxBps: number;
	    netTxBps: number;
	    diskReadBps: number;
	    diskWriteBps: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuPercent = source["cpuPercent"];
	        this.memUsedGB = source["memUsedGB"];
	        this.memFreeGB = source["memFreeGB"];
	        this.memTotalGB = source["memTotalGB"];
	        this.diskUsedGB = source["diskUsedGB"];
	        this.diskFreeGB = source["diskFreeGB"];
	        this.diskTotalGB = source["diskTotalGB"];
	        this.uptimeSecs = source["uptimeSecs"];
	        this.netRxBps = source["netRxBps"];
	        this.netTxBps = source["netTxBps"];
	        this.diskReadBps = source["diskReadBps"];
	        this.diskWriteBps = source["diskWriteBps"];
	    }
	}

}

export namespace models {

	export class DatabaseInfo {
	    name: string;
	    port: number;
	    address: string;
	    source: string;
	    container: string;
	    image: string;

	    static createFrom(source: any = {}) {
	        return new DatabaseInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.address = source["address"];
	        this.source = source["source"];
	        this.container = source["container"];
	        this.image = source["image"];
	    }
	}

	export class Connection {
	    id: string;
	    folderId?: string;
	    name: string;
	    host: string;
	    port: number;
	    username: string;
	    authType: string;
	    password?: string;
	    privateKeyPath?: string;
	    color?: string;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.folderId = source["folderId"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.authType = source["authType"];
	        this.password = source["password"];
	        this.privateKeyPath = source["privateKeyPath"];
	        this.color = source["color"];
	    }
	}
	export class DatabaseInfo {
	    name: string;
	    port: number;
	    address: string;
	    source: string;
	    container: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new DatabaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.address = source["address"];
	        this.source = source["source"];
	        this.container = source["container"];
	        this.image = source["image"];
	    }
	}
	export class DockerContainer {
	    id: string;
	    names: string;
	    image: string;
	    status: string;
	    state: string;
	    ports: string;
	
	    static createFrom(source: any = {}) {
	        return new DockerContainer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.names = source["names"];
	        this.image = source["image"];
	        this.status = source["status"];
	        this.state = source["state"];
	        this.ports = source["ports"];
	    }
	}
	export class Folder {
	    id: string;
	    name: string;
	    parentId?: string;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.parentId = source["parentId"];
	    }
	}
	export class PortInfo {
	    port: number;
	    address: string;
	    process: string;
	    proto: string;
	
	    static createFrom(source: any = {}) {
	        return new PortInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.address = source["address"];
	        this.process = source["process"];
	        this.proto = source["proto"];
	    }
	}
	export class TunnelInfo {
	    id: string;
	    localPort: number;
	    remoteHost: string;
	    remotePort: number;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TunnelInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.localPort = source["localPort"];
	        this.remoteHost = source["remoteHost"];
	        this.remotePort = source["remotePort"];
	        this.active = source["active"];
	    }
	}

}

export namespace sftp {
	
	export class FileItem {
	    name: string;
	    path: string;
	    isDirectory: boolean;
	    size: number;
	    mode: string;
	    // Go type: time
	    modTime: any;
	
	    static createFrom(source: any = {}) {
	        return new FileItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDirectory = source["isDirectory"];
	        this.size = source["size"];
	        this.mode = source["mode"];
	        this.modTime = this.convertValues(source["modTime"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

