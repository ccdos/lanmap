<script lang="ts">
  import { onMount, onDestroy } from "svelte";
  import { GetDefaultSubnet, ScanSubnet, ProbeIP, LoadHistory, SaveHistory } from "../wailsjs/go/main/App.js";
  import { EventsOn, EventsOff } from "../wailsjs/runtime/runtime.js";

  let subnet = "192.168.1.0/24";
  let method = "ARP";
  let isScanning = false;
  let errorMsg = "";
  let progress = 0;
  let totalIps = 0;
  
  // Format: "192.168.1.5" -> { ip: "192.168.1.5", status: "inactive", mac: "" }
  let ipGrid: Array<{ ip: string; status: string; mac: string; lastOctet: string }> = [];

  // History State
  let history: string[] = [];

  // Details Panel State
  let showPanel = false;
  let isProbing = false;
  let selectedIp = "";
  let selectedMac = "";
  let probeResult: any = null;

  onMount(async () => {
    try {
      const saved = await LoadHistory();
      if (saved && saved.length > 0) {
        history = saved;
      } else {
        const localSaved = localStorage.getItem("lanmap_history");
        if (localSaved) history = JSON.parse(localSaved);
      }
    } catch(e) {}

    try {
      subnet = await GetDefaultSubnet();
      initGrid(subnet);
    } catch (e) {
      console.error(e);
    }

    EventsOn("scan_start", (total: number) => {
      isScanning = true;
      errorMsg = "";
      totalIps = total;
      progress = 0;
      ipGrid = ipGrid.map(item => ({ ...item, status: "inactive", mac: "" }));
      showPanel = false; // Hide panel on new scan
    });

    EventsOn("scan_result", (res: any) => {
      progress++;
      const index = ipGrid.findIndex(i => i.ip === res.ip);
      if (index !== -1) {
        ipGrid[index] = { ...ipGrid[index], status: res.status, mac: res.mac };
      }
    });

    EventsOn("scan_error", (err: string) => {
      isScanning = false;
      errorMsg = err;
    });

    EventsOn("scan_complete", () => {
      isScanning = false;
      progress = totalIps;
    });
  });

  onDestroy(() => {
    EventsOff("scan_start");
    EventsOff("scan_result");
    EventsOff("scan_error");
    EventsOff("scan_complete");
  });

  function initGrid(cidr: string) {
    try {
      const parts = cidr.split("/");
      if (parts.length !== 2) return;
      const base = parts[0].split(".").slice(0, 3).join(".");
      
      const newGrid = [];
      for (let i = 1; i < 255; i++) {
        newGrid.push({
          ip: `${base}.${i}`,
          status: "inactive",
          mac: "",
          lastOctet: String(i)
        });
      }
      ipGrid = newGrid;
    } catch (e) {
      console.error("Invalid CIDR format");
    }
  }

  function handleSubnetChange(e: Event) {
    const val = (e.target as HTMLInputElement).value;
    subnet = val;
    initGrid(val);
  }

  function startScan() {
    if (isScanning) return;
    
    // Save to history (keep max 10, move latest to top)
    if (subnet) {
      history = [subnet, ...history.filter(h => h !== subnet)].slice(0, 10);
      localStorage.setItem("lanmap_history", JSON.stringify(history));
      SaveHistory(history);
    }

    initGrid(subnet);
    ScanSubnet(subnet, method);
  }

  async function handleIpClick(item: any) {
    if (item.status !== "active") return;
    
    selectedIp = item.ip;
    selectedMac = item.mac;
    showPanel = true;
    isProbing = true;
    probeResult = null;

    try {
      probeResult = await ProbeIP(item.ip);
    } catch (e) {
      console.error("Probe error:", e);
    } finally {
      isProbing = false;
    }
  }

  function closePanel() {
    showPanel = false;
  }
</script>

<main class="flex flex-col h-screen bg-gray-900 text-gray-100 font-sans relative overflow-hidden">
  <!-- Header Control Bar -->
  <header class="flex flex-wrap items-center justify-between px-6 py-4 bg-gray-800 shadow-lg border-b border-gray-700 select-none relative z-20">
    <div class="flex items-center space-x-6">
      <div class="flex items-center space-x-2">
        <div class="w-3 h-3 bg-emerald-500 rounded-full animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.8)]"></div>
        <div class="font-bold text-xl tracking-widest text-emerald-400 drop-shadow-md">LANMAP</div>
      </div>
      
      <div class="flex items-center space-x-3 bg-gray-900/50 p-1.5 rounded-lg border border-gray-700/50">
        <input 
          type="text" 
          value={subnet} 
          list="subnet-history"
          on:input={handleSubnetChange}
          class="bg-gray-800 border border-gray-600 rounded px-3 py-1.5 text-sm text-gray-200 outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 w-40 transition-colors"
          placeholder="192.168.1.0/24"
        />
        <datalist id="subnet-history">
          {#each history as h}
            <option value={h}></option>
          {/each}
        </datalist>
        
        <div class="relative">
          <select bind:value={method} class="bg-gray-800 border border-gray-600 rounded pl-3 pr-8 py-1.5 text-sm outline-none focus:border-emerald-500 focus:ring-1 focus:ring-emerald-500 cursor-pointer appearance-none transition-colors hover:border-gray-500 text-gray-200">
            <option value="ARP">ARP (Fast/Local)</option>
            <option value="ICMP">ICMP Ping</option>
            <option value="TCP">TCP Port</option>
          </select>
          <div class="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-gray-400">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"></path></svg>
          </div>
        </div>
        
        <button 
          on:click={startScan} 
          disabled={isScanning}
          class="bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white font-medium px-5 py-1.5 rounded transition-all duration-200 disabled:opacity-50 disabled:cursor-not-allowed shadow-md shadow-emerald-900/30 active:scale-95 flex items-center space-x-2"
        >
          {#if isScanning}
            <svg class="animate-spin -ml-1 mr-2 h-4 w-4 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <span>Scanning</span>
          {:else}
            <span>Scan Subnet</span>
          {/if}
        </button>
      </div>
    </div>
    
    <div class="flex items-center space-x-4 text-sm mt-4 md:mt-0">
      {#if errorMsg}
        <div class="px-3 py-1 bg-red-900/40 border border-red-800/50 rounded-md text-red-400 font-medium flex items-center">
          <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          {errorMsg}
        </div>
      {/if}
      <div class="flex flex-col items-end justify-center min-w-[120px] bg-gray-900/40 px-4 py-2 rounded-lg border border-gray-700/30">
        {#if isScanning}
          <div class="w-full mb-1 flex justify-between text-xs font-semibold text-emerald-400">
            <span>Progress</span>
            <span>{Math.round((progress/totalIps)*100)}%</span>
          </div>
          <div class="w-full bg-gray-700 rounded-full h-1.5 overflow-hidden">
            <div class="bg-emerald-500 h-1.5 rounded-full transition-all duration-300 ease-out" style="width: {(progress/totalIps)*100}%"></div>
          </div>
        {:else if progress > 0}
          <span class="text-emerald-400 font-medium">Scanned {totalIps} IPs</span>
          <span class="text-xs text-gray-500">Ready</span>
        {:else}
          <span class="text-gray-400 font-medium">System Ready</span>
          <span class="text-xs text-gray-600">Waiting for scan...</span>
        {/if}
      </div>
    </div>
  </header>

  <!-- Main Grid Area -->
  <div class="flex-1 overflow-auto p-4 md:p-8 flex justify-center custom-scrollbar bg-[url('data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHdpZHRoPSIyMCIgaGVpZ2h0PSIyMCI+PGNpcmNsZSBjeD0iMSIgY3k9IjEiIHI9IjEiIGZpbGw9InJnYmEoMjU1LDI1NSwyNTUsMC4wNSkiLz48L3N2Zz4=')]">
    {#if ipGrid.length > 0}
      <div class="grid grid-cols-16 gap-1.5 md:gap-2.5 h-max p-5 md:p-6 bg-gray-800/80 backdrop-blur-sm rounded-2xl border border-gray-700 shadow-xl self-start transition-all duration-300 {showPanel ? 'mr-[320px]' : ''}">
        {#each ipGrid as item}
          <!-- svelte-ignore a11y-click-events-have-key-events -->
          <!-- svelte-ignore a11y-no-static-element-interactions -->
          <div 
            on:click={() => handleIpClick(item)}
            class="relative flex items-center justify-center w-8 h-8 md:w-12 md:h-12 rounded md:rounded-lg text-[10px] md:text-sm font-semibold transition-all duration-500
                   {item.status === 'active' 
                     ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/60 shadow-[0_0_10px_rgba(16,185,129,0.2)] cursor-pointer hover:bg-emerald-500/30 hover:scale-110 hover:shadow-[0_0_15px_rgba(16,185,129,0.4)] z-10' 
                     : 'bg-gray-900/60 text-gray-600 border border-gray-800 cursor-default hover:bg-gray-700 hover:text-gray-400 hover:border-gray-600'}
                   {selectedIp === item.ip ? 'ring-2 ring-emerald-400 ring-offset-2 ring-offset-gray-800 scale-110' : ''}"
            title={item.status === 'active' ? 'Click to probe details' : `IP: ${item.ip}\nStatus: ${item.status}`}
          >
            {item.lastOctet}
            {#if item.status === 'active'}
              <div class="absolute -top-1 -right-1 w-2 h-2 bg-emerald-400 rounded-full animate-ping opacity-75"></div>
              <div class="absolute -top-1 -right-1 w-2 h-2 bg-emerald-500 rounded-full shadow-[0_0_4px_#10b981]"></div>
            {/if}
          </div>
        {/each}
      </div>
    {:else}
      <div class="flex flex-col items-center justify-center text-gray-500 h-full w-full space-y-4">
        <svg class="w-16 h-16 text-gray-700" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 002-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"></path></svg>
        <div class="text-lg">Please enter a valid Subnet to visualize the grid.</div>
      </div>
    {/if}
  </div>

  <!-- Slide-out Details Panel -->
  <div class="fixed top-0 right-0 w-80 h-full bg-gray-800 shadow-2xl border-l border-gray-700 transform transition-transform duration-300 ease-in-out z-30 pt-[72px]" style="transform: translateX({showPanel ? '0%' : '100%'})">
    <div class="flex flex-col h-full">
      <div class="flex justify-between items-center p-4 border-b border-gray-700">
        <h2 class="text-lg font-semibold text-emerald-400 flex items-center">
          <svg class="w-5 h-5 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path></svg>
          Device Details
        </h2>
        <button on:click={closePanel} class="text-gray-400 hover:text-white transition-colors">
          <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"></path></svg>
        </button>
      </div>
      
      <div class="p-5 flex-1 overflow-y-auto custom-scrollbar">
        <!-- Basic Info (Always available) -->
        <div class="mb-6 space-y-3">
          <div>
            <div class="text-xs text-gray-500 uppercase tracking-wider mb-1">IP Address</div>
            <div class="text-lg font-mono text-gray-200">{selectedIp}</div>
          </div>
          {#if selectedMac}
          <div>
            <div class="text-xs text-gray-500 uppercase tracking-wider mb-1">MAC Address</div>
            <div class="text-sm font-mono text-gray-300 bg-gray-900/50 p-2 rounded border border-gray-700">{selectedMac}</div>
          </div>
          {/if}
        </div>

        <hr class="border-gray-700 mb-6">

        {#if isProbing}
          <div class="flex flex-col items-center justify-center py-10 space-y-4">
            <svg class="animate-spin h-10 w-10 text-emerald-500" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            <div class="text-emerald-400 font-medium animate-pulse">Running Deep Probe...</div>
            <div class="text-xs text-gray-500 text-center max-w-[200px]">Scanning ports, resolving hostname and guessing OS</div>
          </div>
        {:else if probeResult}
          <div class="space-y-5 animate-fade-in">
            <!-- Hostname -->
            <div class="bg-gray-900/40 p-3 rounded-lg border border-gray-700">
              <div class="text-xs text-gray-500 uppercase tracking-wider mb-1">Hostname (DNS)</div>
              <div class="text-sm font-medium {probeResult.hostname !== 'N/A' ? 'text-blue-300' : 'text-gray-400'}">{probeResult.hostname}</div>
            </div>

            <!-- OS Guess -->
            <div class="bg-gray-900/40 p-3 rounded-lg border border-gray-700">
              <div class="text-xs text-gray-500 uppercase tracking-wider mb-1">OS / Device Guess</div>
              <div class="text-sm font-medium text-emerald-300 flex items-center">
                {#if probeResult.os_guess.includes('Windows')}
                  <svg class="w-4 h-4 mr-2" fill="currentColor" viewBox="0 0 24 24"><path d="M0,3.449L9.75,2.101v8.951H0V3.449z M10.649,1.978L24,0.11v10.943H10.649V1.978z M10.649,12.012H24v10.94l-13.351-1.854V12.012z M0,12.012h9.75v8.948L0,19.646V12.012z"/></svg>
                {:else if probeResult.os_guess.includes('Linux')}
                  <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2m-2-4h.01M17 16h.01"></path></svg>
                {/if}
                {probeResult.os_guess}
              </div>
            </div>

            <!-- Latency -->
            <div class="bg-gray-900/40 p-3 rounded-lg border border-gray-700 flex justify-between items-center">
              <span class="text-xs text-gray-500 uppercase tracking-wider">Ping Latency</span>
              <span class="text-sm font-mono text-teal-300">{probeResult.latency}</span>
            </div>

            <!-- Web Title -->
            {#if probeResult.web_title}
              <div class="bg-blue-900/20 p-3 rounded-lg border border-blue-800/50">
                <div class="text-xs text-blue-400/70 uppercase tracking-wider mb-1 flex items-center">
                  <svg class="w-3 h-3 mr-1" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9"></path></svg>
                  Web Page Title
                </div>
                <div class="text-sm font-medium text-blue-200 break-words">{probeResult.web_title}</div>
              </div>
            {/if}

            <!-- Open Ports -->
            <div class="bg-gray-900/40 p-3 rounded-lg border border-gray-700">
              <div class="text-xs text-gray-500 uppercase tracking-wider mb-2">Open Ports</div>
              {#if probeResult.ports && probeResult.ports.length > 0}
                <div class="flex flex-wrap gap-2">
                  {#each probeResult.ports as port}
                    <span class="px-2 py-1 bg-emerald-900/40 border border-emerald-800/60 rounded text-xs font-mono text-emerald-300">
                      {port}
                    </span>
                  {/each}
                </div>
              {:else}
                <div class="text-sm text-gray-500 italic">No common ports detected</div>
              {/if}
            </div>
          </div>
        {/if}
      </div>
    </div>
  </div>
</main>

<style>
  .grid-cols-16 {
    grid-template-columns: repeat(16, minmax(0, 1fr));
  }
  
  .custom-scrollbar::-webkit-scrollbar {
    width: 6px;
    height: 6px;
  }
  .custom-scrollbar::-webkit-scrollbar-track {
    background: transparent; 
  }
  .custom-scrollbar::-webkit-scrollbar-thumb {
    background: #4B5563; 
    border-radius: 3px;
  }
  .custom-scrollbar::-webkit-scrollbar-thumb:hover {
    background: #6B7280; 
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }
  .animate-fade-in {
    animation: fadeIn 0.4s ease-out forwards;
  }
</style>
