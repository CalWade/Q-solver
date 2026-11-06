export namespace application {

	export class Capabilities {
	    filtering: boolean;
	    backend: string;
	    reason?: string;

	    static createFrom(source: any = {}) {
	        return new Capabilities(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.filtering = source["filtering"];
	        this.backend = source["backend"];
	        this.reason = source["reason"];
	    }
	}
	export class Status {
	    state: string;
	    protection: domain.Protection;
	    capabilities: Capabilities;
	    revision: number;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.protection = this.convertValues(source["protection"], domain.Protection);
	        this.capabilities = this.convertValues(source["capabilities"], Capabilities);
	        this.revision = source["revision"];
	        this.error = source["error"];
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

export namespace config {

	export class Config {
	    apiKey?: string;
	    baseURL?: string;
	    model?: string;
	    prompt?: string;
	    domainId?: string;
	    opacity?: number;
	    noCompression?: boolean;
	    compressionQuality?: number;
	    sharpening?: number;
	    grayscale?: boolean;
	    keepContext?: boolean;
	    interruptThinking?: boolean;
	    screenshotMode?: string;
	    resumePath?: string;
	    resumeContent?: string;
	    shortcuts?: Record<string, shortcut.KeyBinding>;
	    keyboardProtection: domain.Protection;
	    assistantModel?: string;
	    windowWidth?: number;
	    windowHeight?: number;
	    theme?: string;

	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.baseURL = source["baseURL"];
	        this.model = source["model"];
	        this.prompt = source["prompt"];
	        this.domainId = source["domainId"];
	        this.opacity = source["opacity"];
	        this.noCompression = source["noCompression"];
	        this.compressionQuality = source["compressionQuality"];
	        this.sharpening = source["sharpening"];
	        this.grayscale = source["grayscale"];
	        this.keepContext = source["keepContext"];
	        this.interruptThinking = source["interruptThinking"];
	        this.screenshotMode = source["screenshotMode"];
	        this.resumePath = source["resumePath"];
	        this.resumeContent = source["resumeContent"];
	        this.shortcuts = this.convertValues(source["shortcuts"], shortcut.KeyBinding, true);
	        this.keyboardProtection = this.convertValues(source["keyboardProtection"], domain.Protection);
	        this.assistantModel = source["assistantModel"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.theme = source["theme"];
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

export namespace domain {

	export class DomainItem {
	    id: string;
	    label: string;
	    icon: string;
	    description: string;

	    static createFrom(source: any = {}) {
	        return new DomainItem(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.icon = source["icon"];
	        this.description = source["description"];
	    }
	}
	export class Category {
	    id: string;
	    label: string;
	    items: DomainItem[];

	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.items = this.convertValues(source["items"], DomainItem);
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

	export class Protection {
	    enabled: boolean;
	    blockLeft: boolean;
	    blockRight: boolean;

	    static createFrom(source: any = {}) {
	        return new Protection(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.blockLeft = source["blockLeft"];
	        this.blockRight = source["blockRight"];
	    }
	}

}

export namespace screen {

	export class PreviewResult {
	    imgBytes: number[];
	    base64: string;
	    size: string;

	    static createFrom(source: any = {}) {
	        return new PreviewResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imgBytes = source["imgBytes"];
	        this.base64 = source["base64"];
	        this.size = source["size"];
	    }
	}

}

export namespace shortcut {

	export class KeyBinding {
	    vkCode: string;
	    keyName: string;
	    keys?: string[];

	    static createFrom(source: any = {}) {
	        return new KeyBinding(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.vkCode = source["vkCode"];
	        this.keyName = source["keyName"];
	        this.keys = source["keys"];
	    }
	}

}

