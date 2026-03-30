export namespace main {
	
	export class Paper {
	    id: string;
	    title: string;
	    authors: string;
	    abstract: string;
	    year: number;
	    journal: string;
	    pdf_path?: string;
	    category: string;
	    tags: string;
	    // Go type: time
	    added_at: any;
	    // Go type: time
	    updated_at: any;
	
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
	        this.pdf_path = source["pdf_path"];
	        this.category = source["category"];
	        this.tags = source["tags"];
	        this.added_at = this.convertValues(source["added_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	    category: string;
	
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
	        this.category = source["category"];
	    }
	}

}

