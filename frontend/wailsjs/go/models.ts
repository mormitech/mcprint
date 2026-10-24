export namespace escl {
	
	export class Resolutions {
	    values: number[];
	    min: number;
	    max: number;
	    step: number;
	
	    static createFrom(source: any = {}) {
	        return new Resolutions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.values = source["values"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.step = source["step"];
	    }
	}
	export class Input {
	    colorModes: string[];
	    formats: string[];
	    resolutions: Resolutions;
	    minWidth: number;
	    maxWidth: number;
	    minHeight: number;
	    maxHeight: number;
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.colorModes = source["colorModes"];
	        this.formats = source["formats"];
	        this.resolutions = this.convertValues(source["resolutions"], Resolutions);
	        this.minWidth = source["minWidth"];
	        this.maxWidth = source["maxWidth"];
	        this.minHeight = source["minHeight"];
	        this.maxHeight = source["maxHeight"];
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
	export class Caps {
	    make: string;
	    model: string;
	    uuid: string;
	    version: string;
	    platen?: Input;
	    simplex?: Input;
	    duplex?: Input;
	    feederCapacity: number;
	
	    static createFrom(source: any = {}) {
	        return new Caps(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.make = source["make"];
	        this.model = source["model"];
	        this.uuid = source["uuid"];
	        this.version = source["version"];
	        this.platen = this.convertValues(source["platen"], Input);
	        this.simplex = this.convertValues(source["simplex"], Input);
	        this.duplex = this.convertValues(source["duplex"], Input);
	        this.feederCapacity = source["feederCapacity"];
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
	export class Choices {
	    sources: string[];
	    canDuplex: boolean;
	    colors: string[];
	    dpis: number[];
	    papers: string[];
	    defaultDpi: number;
	
	    static createFrom(source: any = {}) {
	        return new Choices(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sources = source["sources"];
	        this.canDuplex = source["canDuplex"];
	        this.colors = source["colors"];
	        this.dpis = source["dpis"];
	        this.papers = source["papers"];
	        this.defaultDpi = source["defaultDpi"];
	    }
	}
	export class Device {
	    guid: string;
	    name: string;
	    model: string;
	    host: string;
	    port: number;
	    scheme: string;
	    root: string;
	    // Go type: time
	    lastSeen: any;
	    online: boolean;
	    manual: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Device(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.scheme = source["scheme"];
	        this.root = source["root"];
	        this.lastSeen = this.convertValues(source["lastSeen"], null);
	        this.online = source["online"];
	        this.manual = source["manual"];
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
	
	export class Job {
	    uri: string;
	    state: string;
	    reasons: string[];
	    done: number;
	    toSend: number;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uri = source["uri"];
	        this.state = source["state"];
	        this.reasons = source["reasons"];
	        this.done = source["done"];
	        this.toSend = source["toSend"];
	    }
	}
	
	export class Status {
	    state: string;
	    adfState: string;
	    jobs: Job[];
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.adfState = source["adfState"];
	        this.jobs = this.convertValues(source["jobs"], Job);
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

export namespace main {
	
	export class PageView {
	    id: number;
	    width: number;
	    height: number;
	    dpi: number;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new PageView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.dpi = source["dpi"];
	        this.bytes = source["bytes"];
	    }
	}
	export class ScanRequest {
	    guid: string;
	    source: string;
	    duplex: boolean;
	    dpi: number;
	    color: string;
	    paper: string;
	    insertAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.guid = source["guid"];
	        this.source = source["source"];
	        this.duplex = source["duplex"];
	        this.dpi = source["dpi"];
	        this.color = source["color"];
	        this.paper = source["paper"];
	        this.insertAt = source["insertAt"];
	    }
	}

}

