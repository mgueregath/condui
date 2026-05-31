export namespace models {
	
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

}

