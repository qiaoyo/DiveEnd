export namespace main {
	
	export class BaiduCloudConfig {
	    enabled: boolean;
	    token: string;
	    quota: number;
	
	    static createFrom(source: any = {}) {
	        return new BaiduCloudConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.token = source["token"];
	        this.quota = source["quota"];
	    }
	}
	export class AppConfig {
	    selectedProvider: string;
	    openaiApiKey: string;
	    openaiModel: string;
	    anthropicApiKey: string;
	    anthropicModel: string;
	    semanticScholarApiKey: string;
	    baiduCloud: BaiduCloudConfig;
	    theme: string;
	    leftPanelWidth: number;
	    rightPanelWidth: number;
	    dataPath: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.selectedProvider = source["selectedProvider"];
	        this.openaiApiKey = source["openaiApiKey"];
	        this.openaiModel = source["openaiModel"];
	        this.anthropicApiKey = source["anthropicApiKey"];
	        this.anthropicModel = source["anthropicModel"];
	        this.semanticScholarApiKey = source["semanticScholarApiKey"];
	        this.baiduCloud = this.convertValues(source["baiduCloud"], BaiduCloudConfig);
	        this.theme = source["theme"];
	        this.leftPanelWidth = source["leftPanelWidth"];
	        this.rightPanelWidth = source["rightPanelWidth"];
	        this.dataPath = source["dataPath"];
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
	
	export class Folder {
	    id: string;
	    name: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class Paper {
	    id: string;
	    title: string;
	    authors: string;
	    abstract: string;
	    year: number;
	    journal: string;
	    url: string;
	    pdfPath?: string;
	    folderId: string;
	    category: string;
	    tags: string[];
	    // Go type: time
	    addedAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Paper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.authors = source["authors"];
	        this.abstract = source["abstract"];
	        this.year = source["year"];
	        this.journal = source["journal"];
	        this.url = source["url"];
	        this.pdfPath = source["pdfPath"];
	        this.folderId = source["folderId"];
	        this.category = source["category"];
	        this.tags = source["tags"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class InitialState {
	    config: AppConfig;
	    folders: Folder[];
	    papers: Paper[];
	    activeFolderId: string;
	
	    static createFrom(source: any = {}) {
	        return new InitialState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], AppConfig);
	        this.folders = this.convertValues(source["folders"], Folder);
	        this.papers = this.convertValues(source["papers"], Paper);
	        this.activeFolderId = source["activeFolderId"];
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
	
	export class SaveConfigResult {
	    config: AppConfig;
	    restartRequired: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SaveConfigResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], AppConfig);
	        this.restartRequired = source["restartRequired"];
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
	export class SearchPaper {
	    id: string;
	    title: string;
	    authors: string;
	    abstract: string;
	    year: number;
	    journal: string;
	    url: string;
	    category: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new SearchPaper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.authors = source["authors"];
	        this.abstract = source["abstract"];
	        this.year = source["year"];
	        this.journal = source["journal"];
	        this.url = source["url"];
	        this.category = source["category"];
	        this.tags = source["tags"];
	    }
	}
	export class TranslationRecord {
	    id: string;
	    paperId: string;
	    section: string;
	    originalText: string;
	    translatedText: string;
	    summary: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new TranslationRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.paperId = source["paperId"];
	        this.section = source["section"];
	        this.originalText = source["originalText"];
	        this.translatedText = source["translatedText"];
	        this.summary = source["summary"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

