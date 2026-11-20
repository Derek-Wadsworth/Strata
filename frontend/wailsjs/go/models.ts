export namespace main {
	
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

