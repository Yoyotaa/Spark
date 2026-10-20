export namespace app {
	
	export class KeyInfo {
	    source: string;
	    hint: string;
	    envNames: string[];
	
	    static createFrom(source: any = {}) {
	        return new KeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.hint = source["hint"];
	        this.envNames = source["envNames"];
	    }
	}
	export class AIStatus {
	    ready: boolean;
	    name: string;
	    error?: string;
	    keys: Record<string, KeyInfo>;
	
	    static createFrom(source: any = {}) {
	        return new AIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ready = source["ready"];
	        this.name = source["name"];
	        this.error = source["error"];
	        this.keys = this.convertValues(source["keys"], KeyInfo, true);
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
	export class AppInfo {
	    version: string;
	    platform: string;
	    settingsPath: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.platform = source["platform"];
	        this.settingsPath = source["settingsPath"];
	    }
	}
	export class Capture {
	    width: number;
	    height: number;
	    bytes: number;
	    millis: number;
	    dataUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Capture(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.width = source["width"];
	        this.height = source["height"];
	        this.bytes = source["bytes"];
	        this.millis = source["millis"];
	        this.dataUrl = source["dataUrl"];
	    }
	}
	
	export class MicStatus {
	    mic: string;
	    speech: string;
	    engine: string;
	    live: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MicStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mic = source["mic"];
	        this.speech = source["speech"];
	        this.engine = source["engine"];
	        this.live = source["live"];
	    }
	}
	export class SaveResult {
	    settings: config.Settings;
	    hotkeyError?: string;
	
	    static createFrom(source: any = {}) {
	        return new SaveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.settings = this.convertValues(source["settings"], config.Settings);
	        this.hotkeyError = source["hotkeyError"];
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
	export class TestResult {
	    millis: number;
	    reply: string;
	    model: string;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.millis = source["millis"];
	        this.reply = source["reply"];
	        this.model = source["model"];
	    }
	}
	export class Turn {
	    id: number;
	    capture?: Capture;
	
	    static createFrom(source: any = {}) {
	        return new Turn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.capture = this.convertValues(source["capture"], Capture);
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

export namespace audio {
	
	export class Voice {
	    name: string;
	    lang: string;
	
	    static createFrom(source: any = {}) {
	        return new Voice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.lang = source["lang"];
	    }
	}

}

export namespace config {
	
	export class AISettings {
	    provider: string;
	    models: Record<string, string>;
	    baseUrls: Record<string, string>;
	    thinking: string;
	    length: string;
	    language: string;
	    customInstructions: string;
	    historyTurns: number;
	    fallback: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.models = source["models"];
	        this.baseUrls = source["baseUrls"];
	        this.thinking = source["thinking"];
	        this.length = source["length"];
	        this.language = source["language"];
	        this.customInstructions = source["customInstructions"];
	        this.historyTurns = source["historyTurns"];
	        this.fallback = source["fallback"];
	    }
	}
	export class AdvancedSettings {
	    debug: boolean;
	    pollHz: number;
	
	    static createFrom(source: any = {}) {
	        return new AdvancedSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.debug = source["debug"];
	        this.pollHz = source["pollHz"];
	    }
	}
	export class AppearanceSettings {
	    accent: string;
	    orbSize: string;
	    panelOpacity: number;
	    reduceMotion: boolean;
	    showBubbles: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppearanceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accent = source["accent"];
	        this.orbSize = source["orbSize"];
	        this.panelOpacity = source["panelOpacity"];
	        this.reduceMotion = source["reduceMotion"];
	        this.showBubbles = source["showBubbles"];
	    }
	}
	export class Shortcut {
	    key: string;
	    alt: boolean;
	    shift: boolean;
	    ctrl: boolean;
	    cmd: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Shortcut(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.alt = source["alt"];
	        this.shift = source["shift"];
	        this.ctrl = source["ctrl"];
	        this.cmd = source["cmd"];
	    }
	}
	export class HotkeySettings {
	    toggleChat: Shortcut;
	    toggleVisible: Shortcut;
	    openSettings: Shortcut;
	
	    static createFrom(source: any = {}) {
	        return new HotkeySettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.toggleChat = this.convertValues(source["toggleChat"], Shortcut);
	        this.toggleVisible = this.convertValues(source["toggleVisible"], Shortcut);
	        this.openSettings = this.convertValues(source["openSettings"], Shortcut);
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
	export class MicSettings {
	    mode: string;
	    pushToTalk: Shortcut;
	    sensitivity: number;
	    wakeWord: boolean;
	    wakePhrase: string;
	    cues: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MicSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.pushToTalk = this.convertValues(source["pushToTalk"], Shortcut);
	        this.sensitivity = source["sensitivity"];
	        this.wakeWord = source["wakeWord"];
	        this.wakePhrase = source["wakePhrase"];
	        this.cues = source["cues"];
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
	export class Point {
	    x: number;
	    y: number;
	
	    static createFrom(source: any = {}) {
	        return new Point(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	    }
	}
	export class VoiceSettings {
	    enabled: boolean;
	    voice: string;
	    rate: number;
	    readAnswers: boolean;
	    greeting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VoiceSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.voice = source["voice"];
	        this.rate = source["rate"];
	        this.readAnswers = source["readAnswers"];
	        this.greeting = source["greeting"];
	    }
	}
	export class VisionSettings {
	    enabled: boolean;
	    maxSide: number;
	    quality: number;
	    showCursor: boolean;
	
	    static createFrom(source: any = {}) {
	        return new VisionSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.maxSide = source["maxSide"];
	        this.quality = source["quality"];
	        this.showCursor = source["showCursor"];
	    }
	}
	export class Settings {
	    orb?: Point;
	    ai: AISettings;
	    vision: VisionSettings;
	    voice: VoiceSettings;
	    mic: MicSettings;
	    hotkeys: HotkeySettings;
	    appearance: AppearanceSettings;
	    advanced: AdvancedSettings;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.orb = this.convertValues(source["orb"], Point);
	        this.ai = this.convertValues(source["ai"], AISettings);
	        this.vision = this.convertValues(source["vision"], VisionSettings);
	        this.voice = this.convertValues(source["voice"], VoiceSettings);
	        this.mic = this.convertValues(source["mic"], MicSettings);
	        this.hotkeys = this.convertValues(source["hotkeys"], HotkeySettings);
	        this.appearance = this.convertValues(source["appearance"], AppearanceSettings);
	        this.advanced = this.convertValues(source["advanced"], AdvancedSettings);
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

export namespace overlay {
	
	export class Rect {
	    x: number;
	    y: number;
	    w: number;
	    h: number;
	
	    static createFrom(source: any = {}) {
	        return new Rect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.x = source["x"];
	        this.y = source["y"];
	        this.w = source["w"];
	        this.h = source["h"];
	    }
	}

}

