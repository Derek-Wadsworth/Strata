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

export namespace stats {
	
	export class Session {
	    // Go type: time
	    start: any;
	    // Go type: time
	    end: any;
	    saves: number;
	    minutes: number;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.start = this.convertValues(source["start"], null);
	        this.end = this.convertValues(source["end"], null);
	        this.saves = source["saves"];
	        this.minutes = source["minutes"];
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
	export class FileActivity {
	    path: string;
	    name: string;
	    versions: number;
	
	    static createFrom(source: any = {}) {
	        return new FileActivity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.versions = source["versions"];
	    }
	}
	export class SizePoint {
	    // Go type: time
	    at: any;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new SizePoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.size = source["size"];
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
	export class DayCount {
	    day: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new DayCount(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.count = source["count"];
	    }
	}
	export class FileStats {
	    path: string;
	    name: string;
	    totalVersions: number;
	    // Go type: time
	    firstEdited: any;
	    // Go type: time
	    lastEdited: any;
	    linesAdded: number;
	    linesRemoved: number;
	    netGrowth: number;
	    editsPerDay: DayCount[];
	    sizeOverTime: SizePoint[];
	
	    static createFrom(source: any = {}) {
	        return new FileStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.totalVersions = source["totalVersions"];
	        this.firstEdited = this.convertValues(source["firstEdited"], null);
	        this.lastEdited = this.convertValues(source["lastEdited"], null);
	        this.linesAdded = source["linesAdded"];
	        this.linesRemoved = source["linesRemoved"];
	        this.netGrowth = source["netGrowth"];
	        this.editsPerDay = this.convertValues(source["editsPerDay"], DayCount);
	        this.sizeOverTime = this.convertValues(source["sizeOverTime"], SizePoint);
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
	export class Dashboard {
	    totalFiles: number;
	    totalVersions: number;
	    linesAdded: number;
	    linesRemoved: number;
	    estimatedActiveMinutes: number;
	    sessionCount: number;
	    files: FileStats[];
	    mostActive: FileActivity[];
	    editsPerDay: DayCount[];
	    heatmap: DayCount[];
	    sessions: Session[];
	
	    static createFrom(source: any = {}) {
	        return new Dashboard(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalFiles = source["totalFiles"];
	        this.totalVersions = source["totalVersions"];
	        this.linesAdded = source["linesAdded"];
	        this.linesRemoved = source["linesRemoved"];
	        this.estimatedActiveMinutes = source["estimatedActiveMinutes"];
	        this.sessionCount = source["sessionCount"];
	        this.files = this.convertValues(source["files"], FileStats);
	        this.mostActive = this.convertValues(source["mostActive"], FileActivity);
	        this.editsPerDay = this.convertValues(source["editsPerDay"], DayCount);
	        this.heatmap = this.convertValues(source["heatmap"], DayCount);
	        this.sessions = this.convertValues(source["sessions"], Session);
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

