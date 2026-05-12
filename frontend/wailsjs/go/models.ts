export namespace main {
	
	export class BaiduCloudConfig {
	    enabled: boolean;
	    token?: string;
	    refreshToken?: string;
	    clientId?: string;
	    clientSecret?: string;
	    hasToken: boolean;
	    quota: number;
	    clearToken?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BaiduCloudConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.token = source["token"];
	        this.refreshToken = source["refreshToken"];
	        this.clientId = source["clientId"];
	        this.clientSecret = source["clientSecret"];
	        this.hasToken = source["hasToken"];
	        this.quota = source["quota"];
	        this.clearToken = source["clearToken"];
	    }
	}
	export class SearchAPIConfig {
	    enableSemanticScholar: boolean;
	    enableArxiv: boolean;
	    semanticScholarKeyPath: string;
	    perSourceResultLimit: number;
	    deepStartResultLimit: number;
	    retryDurationSeconds: number;
	    retryIntervalSeconds: number;
	    requestTimeoutSeconds: number;
	    semanticScholarApiKey?: string;
	    hasSemanticScholarApiKey: boolean;
	    clearSemanticScholarApiKey?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SearchAPIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enableSemanticScholar = source["enableSemanticScholar"];
	        this.enableArxiv = source["enableArxiv"];
	        this.semanticScholarKeyPath = source["semanticScholarKeyPath"];
	        this.perSourceResultLimit = source["perSourceResultLimit"];
	        this.deepStartResultLimit = source["deepStartResultLimit"];
	        this.retryDurationSeconds = source["retryDurationSeconds"];
	        this.retryIntervalSeconds = source["retryIntervalSeconds"];
	        this.requestTimeoutSeconds = source["requestTimeoutSeconds"];
	        this.semanticScholarApiKey = source["semanticScholarApiKey"];
	        this.hasSemanticScholarApiKey = source["hasSemanticScholarApiKey"];
	        this.clearSemanticScholarApiKey = source["clearSemanticScholarApiKey"];
	    }
	}
	export class LLMConfig {
	    providerId: string;
	    providerName: string;
	    providerType: string;
	    baseUrl: string;
	    wireApi: string;
	    requiresOpenAIAuth: boolean;
	    apiKey?: string;
	    hasApiKey: boolean;
	    model: string;
	    reasoningEffort: string;
	    disableResponseStorage: boolean;
	    clearApiKey?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.providerId = source["providerId"];
	        this.providerName = source["providerName"];
	        this.providerType = source["providerType"];
	        this.baseUrl = source["baseUrl"];
	        this.wireApi = source["wireApi"];
	        this.requiresOpenAIAuth = source["requiresOpenAIAuth"];
	        this.apiKey = source["apiKey"];
	        this.hasApiKey = source["hasApiKey"];
	        this.model = source["model"];
	        this.reasoningEffort = source["reasoningEffort"];
	        this.disableResponseStorage = source["disableResponseStorage"];
	        this.clearApiKey = source["clearApiKey"];
	    }
	}
	export class AppConfig {
	    llm: LLMConfig;
	    weakLLM: LLMConfig;
	    search: SearchAPIConfig;
	    baiduCloud: BaiduCloudConfig;
	    theme: string;
	    leftPanelWidth: number;
	    rightPanelWidth: number;
	    dataPath: string;
	    selectedProvider?: string;
	    openaiApiKey?: string;
	    openaiModel?: string;
	    anthropicApiKey?: string;
	    anthropicModel?: string;
	    semanticScholarApiKey?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.llm = this.convertValues(source["llm"], LLMConfig);
	        this.weakLLM = this.convertValues(source["weakLLM"], LLMConfig);
	        this.search = this.convertValues(source["search"], SearchAPIConfig);
	        this.baiduCloud = this.convertValues(source["baiduCloud"], BaiduCloudConfig);
	        this.theme = source["theme"];
	        this.leftPanelWidth = source["leftPanelWidth"];
	        this.rightPanelWidth = source["rightPanelWidth"];
	        this.dataPath = source["dataPath"];
	        this.selectedProvider = source["selectedProvider"];
	        this.openaiApiKey = source["openaiApiKey"];
	        this.openaiModel = source["openaiModel"];
	        this.anthropicApiKey = source["anthropicApiKey"];
	        this.anthropicModel = source["anthropicModel"];
	        this.semanticScholarApiKey = source["semanticScholarApiKey"];
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
	
	export class ConfigSecretPrefill {
	    strongLLMApiKey?: string;
	    hasStrongLLMApiKey: boolean;
	    weakLLMApiKey?: string;
	    hasWeakLLMApiKey: boolean;
	    baiduToken?: string;
	    hasBaiduToken: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConfigSecretPrefill(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strongLLMApiKey = source["strongLLMApiKey"];
	        this.hasStrongLLMApiKey = source["hasStrongLLMApiKey"];
	        this.weakLLMApiKey = source["weakLLMApiKey"];
	        this.hasWeakLLMApiKey = source["hasWeakLLMApiKey"];
	        this.baiduToken = source["baiduToken"];
	        this.hasBaiduToken = source["hasBaiduToken"];
	    }
	}
	export class CreateFolderNodeRequest {
	    parentId?: string;
	    path?: string;
	    name?: string;
	
	    static createFrom(source: any = {}) {
	        return new CreateFolderNodeRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.parentId = source["parentId"];
	        this.path = source["path"];
	        this.name = source["name"];
	    }
	}
	export class DeepReadNote {
	    id: string;
	    paperId: string;
	    section: string;
	    content: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeepReadNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.paperId = source["paperId"];
	        this.section = source["section"];
	        this.content = source["content"];
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
	export class DeepReadSection {
	    id: string;
	    title: string;
	    content: string;
	    index: number;
	
	    static createFrom(source: any = {}) {
	        return new DeepReadSection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.index = source["index"];
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
	export class DeepReadState {
	    paperId: string;
	    hasPdf: boolean;
	    pdfPath: string;
	    parseStatus: string;
	    parseError?: string;
	    sections: DeepReadSection[];
	    markdown?: string;
	    translations: TranslationRecord[];
	    notes: DeepReadNote[];
	    // Go type: time
	    lastPreparedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeepReadState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paperId = source["paperId"];
	        this.hasPdf = source["hasPdf"];
	        this.pdfPath = source["pdfPath"];
	        this.parseStatus = source["parseStatus"];
	        this.parseError = source["parseError"];
	        this.sections = this.convertValues(source["sections"], DeepReadSection);
	        this.markdown = source["markdown"];
	        this.translations = this.convertValues(source["translations"], TranslationRecord);
	        this.notes = this.convertValues(source["notes"], DeepReadNote);
	        this.lastPreparedAt = this.convertValues(source["lastPreparedAt"], null);
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
	export class SearchRetrievalStats {
	    query: string;
	    originalQuery?: string;
	    rewrittenQueries?: string[];
	    queryHits?: Record<string, number>;
	    rawCount: number;
	    dedupCount: number;
	    finalCount: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchRetrievalStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.originalQuery = source["originalQuery"];
	        this.rewrittenQueries = source["rewrittenQueries"];
	        this.queryHits = source["queryHits"];
	        this.rawCount = source["rawCount"];
	        this.dedupCount = source["dedupCount"];
	        this.finalCount = source["finalCount"];
	    }
	}
	export class DeepStartPaperNote {
	    paperId: string;
	    tier: string;
	    reason: string;
	    directionIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new DeepStartPaperNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.paperId = source["paperId"];
	        this.tier = source["tier"];
	        this.reason = source["reason"];
	        this.directionIds = source["directionIds"];
	    }
	}
	export class DeepStartDirection {
	    id: string;
	    name: string;
	    summary: string;
	    why: string;
	    paperIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new DeepStartDirection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.summary = source["summary"];
	        this.why = source["why"];
	        this.paperIds = source["paperIds"];
	    }
	}
	export class DeepStartAnalysis {
	    overview: string;
	    directions: DeepStartDirection[];
	    paperNotes: DeepStartPaperNote[];
	    followUpQuestions: string[];
	    suggestedQueries: string[];
	    recommendedPaperIds: string[];
	    retainedPaperIds: string[];
	    searchStats: SearchRetrievalStats;
	
	    static createFrom(source: any = {}) {
	        return new DeepStartAnalysis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.overview = source["overview"];
	        this.directions = this.convertValues(source["directions"], DeepStartDirection);
	        this.paperNotes = this.convertValues(source["paperNotes"], DeepStartPaperNote);
	        this.followUpQuestions = source["followUpQuestions"];
	        this.suggestedQueries = source["suggestedQueries"];
	        this.recommendedPaperIds = source["recommendedPaperIds"];
	        this.retainedPaperIds = source["retainedPaperIds"];
	        this.searchStats = this.convertValues(source["searchStats"], SearchRetrievalStats);
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
	
	export class DeepStartMessage {
	    id: string;
	    sessionId: string;
	    role: string;
	    content: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeepStartMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.role = source["role"];
	        this.content = source["content"];
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
	
	export class SearchPaper {
	    id: string;
	    title: string;
	    authors: string;
	    abstract: string;
	    year: number;
	    journal: string;
	    publicationVenue: string;
	    publicationYear: number;
	    citationCount: number;
	    url: string;
	    category: string;
	    tags: string[];
	    source: string;
	    externalIds?: Record<string, string>;
	    pdfCandidates?: string[];
	    institutions: string[];
	    keywords: string[];
	    sourceLabel: string;
	    enrichmentNote?: string;
	    preprocessStatus?: string;
	    localPdfPath?: string;
	    markdownPath?: string;
	    parseStatus?: string;
	    parseError?: string;
	    extractStatus?: string;
	    extractError?: string;
	    problem?: string;
	    method?: string;
	    topicLabel?: string;
	    methodLabel?: string;
	    taskLabel?: string;
	    domainLabel?: string;
	    classificationConfidence?: number;
	    processingStage?: string;
	    processingError?: string;
	
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
	        this.publicationVenue = source["publicationVenue"];
	        this.publicationYear = source["publicationYear"];
	        this.citationCount = source["citationCount"];
	        this.url = source["url"];
	        this.category = source["category"];
	        this.tags = source["tags"];
	        this.source = source["source"];
	        this.externalIds = source["externalIds"];
	        this.pdfCandidates = source["pdfCandidates"];
	        this.institutions = source["institutions"];
	        this.keywords = source["keywords"];
	        this.sourceLabel = source["sourceLabel"];
	        this.enrichmentNote = source["enrichmentNote"];
	        this.preprocessStatus = source["preprocessStatus"];
	        this.localPdfPath = source["localPdfPath"];
	        this.markdownPath = source["markdownPath"];
	        this.parseStatus = source["parseStatus"];
	        this.parseError = source["parseError"];
	        this.extractStatus = source["extractStatus"];
	        this.extractError = source["extractError"];
	        this.problem = source["problem"];
	        this.method = source["method"];
	        this.topicLabel = source["topicLabel"];
	        this.methodLabel = source["methodLabel"];
	        this.taskLabel = source["taskLabel"];
	        this.domainLabel = source["domainLabel"];
	        this.classificationConfidence = source["classificationConfidence"];
	        this.processingStage = source["processingStage"];
	        this.processingError = source["processingError"];
	    }
	}
	export class DeepStartSessionSummary {
	    id: string;
	    title: string;
	    rootPrompt: string;
	    currentQuery: string;
	    targetFolderId: string;
	    processingStatus?: string;
	    initialReadyCount?: number;
	    totalPlannedCount?: number;
	    backgroundRemaining?: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeepStartSessionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.rootPrompt = source["rootPrompt"];
	        this.currentQuery = source["currentQuery"];
	        this.targetFolderId = source["targetFolderId"];
	        this.processingStatus = source["processingStatus"];
	        this.initialReadyCount = source["initialReadyCount"];
	        this.totalPlannedCount = source["totalPlannedCount"];
	        this.backgroundRemaining = source["backgroundRemaining"];
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
	export class DeepStartSessionDetail {
	    summary: DeepStartSessionSummary;
	    messages: DeepStartMessage[];
	    currentResults: SearchPaper[];
	    currentAnalysis?: DeepStartAnalysis;
	    selectedPaperIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new DeepStartSessionDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = this.convertValues(source["summary"], DeepStartSessionSummary);
	        this.messages = this.convertValues(source["messages"], DeepStartMessage);
	        this.currentResults = this.convertValues(source["currentResults"], SearchPaper);
	        this.currentAnalysis = this.convertValues(source["currentAnalysis"], DeepStartAnalysis);
	        this.selectedPaperIds = source["selectedPaperIds"];
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
	
	export class SearchSourceStatus {
	    name: string;
	    success: boolean;
	    error?: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new SearchSourceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.success = source["success"];
	        this.error = source["error"];
	        this.count = source["count"];
	    }
	}
	export class EnhancedSearchResult {
	    papers: SearchPaper[];
	    total: number;
	    hasMore: boolean;
	    sources: SearchSourceStatus[];
	    query: string;
	    limit: number;
	    offset: number;
	    yearStart?: number;
	    yearEnd?: number;
	    sortBy?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnhancedSearchResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.papers = this.convertValues(source["papers"], SearchPaper);
	        this.total = source["total"];
	        this.hasMore = source["hasMore"];
	        this.sources = this.convertValues(source["sources"], SearchSourceStatus);
	        this.query = source["query"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.yearStart = source["yearStart"];
	        this.yearEnd = source["yearEnd"];
	        this.sortBy = source["sortBy"];
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
	export class ExtractProgress {
	    sessionId: string;
	    total: number;
	    completed: number;
	    currentFile: string;
	    status: string;
	    errorMessage?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExtractProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.currentFile = source["currentFile"];
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	    }
	}
	export class Folder {
	    id: string;
	    name: string;
	    parentId?: string;
	    path: string;
	    isSystem: boolean;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.parentId = source["parentId"];
	        this.path = source["path"];
	        this.isSystem = source["isSystem"];
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
	export class FolderNode {
	    folder: Folder;
	    children: FolderNode[];
	
	    static createFrom(source: any = {}) {
	        return new FolderNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folder = this.convertValues(source["folder"], Folder);
	        this.children = this.convertValues(source["children"], FolderNode);
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
	export class FolderStorageTreeNode {
	    folderId: string;
	    folderName: string;
	    folderPath: string;
	    paperCount: number;
	    queued: number;
	    downloading: number;
	    downloaded: number;
	    failed: number;
	    children: FolderStorageTreeNode[];
	
	    static createFrom(source: any = {}) {
	        return new FolderStorageTreeNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderId = source["folderId"];
	        this.folderName = source["folderName"];
	        this.folderPath = source["folderPath"];
	        this.paperCount = source["paperCount"];
	        this.queued = source["queued"];
	        this.downloading = source["downloading"];
	        this.downloaded = source["downloaded"];
	        this.failed = source["failed"];
	        this.children = this.convertValues(source["children"], FolderStorageTreeNode);
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
	export class FolderStorageTreeOverview {
	    rootPath: string;
	    directories: FolderStorageTreeNode[];
	    // Go type: time
	    generatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new FolderStorageTreeOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootPath = source["rootPath"];
	        this.directories = this.convertValues(source["directories"], FolderStorageTreeNode);
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
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
	export class ImportSkippedPaper {
	    sourcePaperId: string;
	    title: string;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportSkippedPaper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePaperId = source["sourcePaperId"];
	        this.title = source["title"];
	        this.reason = source["reason"];
	    }
	}
	export class Paper {
	    id: string;
	    sourcePaperId: string;
	    title: string;
	    authors: string;
	    abstract: string;
	    year: number;
	    journal: string;
	    url: string;
	    pdfPath?: string;
	    downloadStatus: string;
	    downloadError?: string;
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
	        this.sourcePaperId = source["sourcePaperId"];
	        this.title = source["title"];
	        this.authors = source["authors"];
	        this.abstract = source["abstract"];
	        this.year = source["year"];
	        this.journal = source["journal"];
	        this.url = source["url"];
	        this.pdfPath = source["pdfPath"];
	        this.downloadStatus = source["downloadStatus"];
	        this.downloadError = source["downloadError"];
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
	export class ImportPapersWithAssetsResult {
	    imported: Paper[];
	    skipped: ImportSkippedPaper[];
	    queued: number;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new ImportPapersWithAssetsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = this.convertValues(source["imported"], Paper);
	        this.skipped = this.convertValues(source["skipped"], ImportSkippedPaper);
	        this.queued = source["queued"];
	        this.message = source["message"];
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
	    deepstartSessions: DeepStartSessionSummary[];
	    activeDeepStartSession?: DeepStartSessionDetail;
	
	    static createFrom(source: any = {}) {
	        return new InitialState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.config = this.convertValues(source["config"], AppConfig);
	        this.folders = this.convertValues(source["folders"], Folder);
	        this.papers = this.convertValues(source["papers"], Paper);
	        this.activeFolderId = source["activeFolderId"];
	        this.deepstartSessions = this.convertValues(source["deepstartSessions"], DeepStartSessionSummary);
	        this.activeDeepStartSession = this.convertValues(source["activeDeepStartSession"], DeepStartSessionDetail);
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
	
	export class LocalStorageFolderOverview {
	    folderId: string;
	    folderName: string;
	    folderPath: string;
	    totalPapers: number;
	    queued: number;
	    downloading: number;
	    downloaded: number;
	    failed: number;
	    storedFileCount: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalStorageFolderOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folderId = source["folderId"];
	        this.folderName = source["folderName"];
	        this.folderPath = source["folderPath"];
	        this.totalPapers = source["totalPapers"];
	        this.queued = source["queued"];
	        this.downloading = source["downloading"];
	        this.downloaded = source["downloaded"];
	        this.failed = source["failed"];
	        this.storedFileCount = source["storedFileCount"];
	    }
	}
	export class LocalStorageOverview {
	    rootPath: string;
	    totalFolders: number;
	    totalFiles: number;
	    queued: number;
	    downloading: number;
	    downloaded: number;
	    failed: number;
	    folders: LocalStorageFolderOverview[];
	    // Go type: time
	    generatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new LocalStorageOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rootPath = source["rootPath"];
	        this.totalFolders = source["totalFolders"];
	        this.totalFiles = source["totalFiles"];
	        this.queued = source["queued"];
	        this.downloading = source["downloading"];
	        this.downloaded = source["downloaded"];
	        this.failed = source["failed"];
	        this.folders = this.convertValues(source["folders"], LocalStorageFolderOverview);
	        this.generatedAt = this.convertValues(source["generatedAt"], null);
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
	
	export class PathHistoryItem {
	    dimension: string;
	    choice: string;
	
	    static createFrom(source: any = {}) {
	        return new PathHistoryItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dimension = source["dimension"];
	        this.choice = source["choice"];
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
	export class ScreeningDecisionOption {
	    key: string;
	    label: string;
	    paperIds: string[];
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new ScreeningDecisionOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.label = source["label"];
	        this.paperIds = source["paperIds"];
	        this.count = source["count"];
	    }
	}
	export class ScreeningDecisionNode {
	    id: string;
	    nodeType: string;
	    message: string;
	    dimension: string;
	    options: ScreeningDecisionOption[];
	    allowMultiSelect: boolean;
	    allowSkip: boolean;
	    remainingPaperIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new ScreeningDecisionNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nodeType = source["nodeType"];
	        this.message = source["message"];
	        this.dimension = source["dimension"];
	        this.options = this.convertValues(source["options"], ScreeningDecisionOption);
	        this.allowMultiSelect = source["allowMultiSelect"];
	        this.allowSkip = source["allowSkip"];
	        this.remainingPaperIds = source["remainingPaperIds"];
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
	
	export class ScreeningPaper {
	    id: string;
	    sessionId: string;
	    fileName: string;
	    filePath: string;
	    fileSize: number;
	    status: string;
	    title?: string;
	    authors?: string;
	    abstract?: string;
	    fullText?: string;
	    sectionsJson?: string;
	    selection?: string;
	    reason?: string;
	    targetFolderId?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ScreeningPaper(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sessionId = source["sessionId"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.fileSize = source["fileSize"];
	        this.status = source["status"];
	        this.title = source["title"];
	        this.authors = source["authors"];
	        this.abstract = source["abstract"];
	        this.fullText = source["fullText"];
	        this.sectionsJson = source["sectionsJson"];
	        this.selection = source["selection"];
	        this.reason = source["reason"];
	        this.targetFolderId = source["targetFolderId"];
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
	export class ScreeningSession {
	    id: string;
	    title: string;
	    status: string;
	    totalPapers: number;
	    currentNodeJson?: string;
	    selectedOptionsJson?: string;
	    pathHistoryJson?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ScreeningSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.totalPapers = source["totalPapers"];
	        this.currentNodeJson = source["currentNodeJson"];
	        this.selectedOptionsJson = source["selectedOptionsJson"];
	        this.pathHistoryJson = source["pathHistoryJson"];
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
	export class ScreeningSessionDetail {
	    session: ScreeningSession;
	    papers: ScreeningPaper[];
	    currentNode?: ScreeningDecisionNode;
	    pathHistory: PathHistoryItem[];
	
	    static createFrom(source: any = {}) {
	        return new ScreeningSessionDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.session = this.convertValues(source["session"], ScreeningSession);
	        this.papers = this.convertValues(source["papers"], ScreeningPaper);
	        this.currentNode = this.convertValues(source["currentNode"], ScreeningDecisionNode);
	        this.pathHistory = this.convertValues(source["pathHistory"], PathHistoryItem);
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
	
	
	
	
	export class SyncConflict {
	    id: string;
	    fileName: string;
	    localPath: string;
	    // Go type: time
	    localTime: any;
	    remotePath: string;
	    // Go type: time
	    remoteTime: any;
	    resolution: string;
	    // Go type: time
	    resolvedAt?: any;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new SyncConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.localPath = source["localPath"];
	        this.localTime = this.convertValues(source["localTime"], null);
	        this.remotePath = source["remotePath"];
	        this.remoteTime = this.convertValues(source["remoteTime"], null);
	        this.resolution = source["resolution"];
	        this.resolvedAt = this.convertValues(source["resolvedAt"], null);
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
	export class SyncProgress {
	    total: number;
	    completed: number;
	    currentFile: string;
	    status: string;
	    message?: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncProgress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.completed = source["completed"];
	        this.currentFile = source["currentFile"];
	        this.status = source["status"];
	        this.message = source["message"];
	    }
	}
	export class SyncRecord {
	    id: string;
	    type: string;
	    fileName: string;
	    fileSize: number;
	    remotePath: string;
	    localPath: string;
	    status: string;
	    errorMessage?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    completedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new SyncRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.fileName = source["fileName"];
	        this.fileSize = source["fileSize"];
	        this.remotePath = source["remotePath"];
	        this.localPath = source["localPath"];
	        this.status = source["status"];
	        this.errorMessage = source["errorMessage"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.completedAt = this.convertValues(source["completedAt"], null);
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
	export class SyncStatus {
	    enabled: boolean;
	    provider: string;
	    // Go type: time
	    lastSync?: any;
	    syncInProgress: boolean;
	    pendingFiles: number;
	    conflicts: number;
	    totalSynced: number;
	    totalFailed: number;
	
	    static createFrom(source: any = {}) {
	        return new SyncStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.lastSync = this.convertValues(source["lastSync"], null);
	        this.syncInProgress = source["syncInProgress"];
	        this.pendingFiles = source["pendingFiles"];
	        this.conflicts = source["conflicts"];
	        this.totalSynced = source["totalSynced"];
	        this.totalFailed = source["totalFailed"];
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

