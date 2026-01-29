export namespace debate {
	
	export class Manager {
	
	
	    static createFrom(source: any = {}) {
	        return new Manager(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace main {
	
	export class AgentInfoJS {
	    id: string;
	    name: string;
	    role: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentInfoJS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.color = source["color"];
	    }
	}
	export class AgentYAMLConfig {
	    id: string;
	    name: string;
	    role: string;
	    system_prompt: string;
	    provider: string;
	    model: string;
	    color: string;
	    api_key?: string;
	    base_url?: string;
	    temperature: number;
	    max_tokens: number;
	    top_p: number;
	    top_k: number;
	    frequency_penalty: number;
	    presence_penalty: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentYAMLConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.role = source["role"];
	        this.system_prompt = source["system_prompt"];
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.color = source["color"];
	        this.api_key = source["api_key"];
	        this.base_url = source["base_url"];
	        this.temperature = source["temperature"];
	        this.max_tokens = source["max_tokens"];
	        this.top_p = source["top_p"];
	        this.top_k = source["top_k"];
	        this.frequency_penalty = source["frequency_penalty"];
	        this.presence_penalty = source["presence_penalty"];
	    }
	}
	export class DebateStatusJS {
	    is_running: boolean;
	    topic: string;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new DebateStatusJS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.is_running = source["is_running"];
	        this.topic = source["topic"];
	        this.mode = source["mode"];
	    }
	}
	export class MessageJS {
	    id: string;
	    agent_id: string;
	    agent_name: string;
	    content: string;
	    timestamp: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new MessageJS(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.agent_id = source["agent_id"];
	        this.agent_name = source["agent_name"];
	        this.content = source["content"];
	        this.timestamp = source["timestamp"];
	        this.color = source["color"];
	    }
	}

}

