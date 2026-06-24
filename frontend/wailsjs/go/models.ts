export namespace main {
	
	export class ProbeResult {
	    ip: string;
	    hostname: string;
	    latency: string;
	    os_guess: string;
	    ports: string[];
	    web_title: string;
	
	    static createFrom(source: any = {}) {
	        return new ProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.hostname = source["hostname"];
	        this.latency = source["latency"];
	        this.os_guess = source["os_guess"];
	        this.ports = source["ports"];
	        this.web_title = source["web_title"];
	    }
	}

}

