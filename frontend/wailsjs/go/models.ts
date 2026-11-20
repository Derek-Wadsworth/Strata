export namespace main {
	
	export class DiffLineInfo {
	    op: string;
	    text: string;
	    oldNo: number;
	    newNo: number;
	
	    static createFrom(source: any = {}) {
	        return new DiffLineInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.op = source["op"];
	        this.text = source["text"];
	        this.oldNo = source["oldNo"];
	        this.newNo = source["newNo"];
	    }
	}
	export class DiffInfo {
	    path: string;
	    fromVer: number;
	    toVer: number;
	    binary: boolean;
	    linesAdded: number;
	    linesRemoved: number;
	    lines: DiffLineInfo[];
	
	    static createFrom(source: any = {}) {
	        return new DiffInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fromVer = source["fromVer"];
	        this.toVer = source["toVer"];
	        this.binary = source["binary"];
	        this.linesAdded = source["linesAdded"];
	        this.linesRemoved = source["linesRemoved"];
	        this.lines = this.convertValues(source["lines"], DiffLineInfo);
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
	
	export class FileInfo {
	    path: string;
	    versionCount: number;
	    missing: boolean;
	    addedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.versionCount = source["versionCount"];
	        this.missing = source["missing"];
	        this.addedAt = source["addedAt"];
	    }
	}
	export class RestoreInfo {
	    path: string;
	    restoredVersion: number;
	    createdSafetyNet: boolean;
	    safetyVersionNum: number;
	
	    static createFrom(source: any = {}) {
	        return new RestoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.restoredVersion = source["restoredVersion"];
	        this.createdSafetyNet = source["createdSafetyNet"];
	        this.safetyVersionNum = source["safetyVersionNum"];
	    }
	}
	export class VersionInfo {
	    number: number;
	    createdAt: string;
	    size: number;
	    linesAdded?: number;
	    linesRemoved?: number;
	
	    static createFrom(source: any = {}) {
	        return new VersionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.createdAt = source["createdAt"];
	        this.size = source["size"];
	        this.linesAdded = source["linesAdded"];
	        this.linesRemoved = source["linesRemoved"];
	    }
	}

}

