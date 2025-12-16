// --- 1. Inicializálás és Loading kártya kezelés ---
document.addEventListener("DOMContentLoaded", function() {
    setupToastUI();
    
    // Azonnal ellenőrizzük, kell-e loading kártyát mutatni
    showLoadingCardIfActive();
});

// --- ÚJ FÜGGVÉNY: Loading Kártya renderelése ---
function showLoadingCardIfActive() {
    const activeJobId = localStorage.getItem('active_job_id');
    const listContainer = document.querySelector('.recipe-list');

    // Csak akkor fut, ha van aktív munka ÉS van recept lista az oldalon (tehát a /receptek oldalon vagyunk)
    if (activeJobId && listContainer) {
        
        // Ellenőrizzük, nincs-e már kint (hogy ne duplikáljuk)
        if (document.getElementById('temp-loading-card')) return;

        // A Skeleton HTML szerkezete
        const loadingHTML = `
            <article class="recipe-card loading-card" id="temp-loading-card">
                <div class="recipe-header" style="display: flex; align-items: center;">
                    <div class="loading-spinner"></div>
                    <div class="recipe-title">Recept generálása...</div>
                </div>
                <div class="recipe-body">
                    <div class="section-title">🛒 Hozzávalók</div>
                    <div class="ingredients-list">
                        <div class="skeleton-box skeleton-tag"></div>
                        <div class="skeleton-box skeleton-tag"></div>
                        <div class="skeleton-box skeleton-tag"></div>
                    </div>
                    <hr style="border: 0; border-top: 1px solid #e2e8f0; margin: 1rem 0;">
                    <div class="section-title">👨‍🍳 Elkészítés</div>
                    <div class="skeleton-box skeleton-text"></div>
                    <div class="skeleton-box skeleton-text"></div>
                    <div class="skeleton-box skeleton-text short"></div>
                </div>
            </article>
        `;

        // Beszúrjuk a lista ELEJÉRE (prepend), hogy legfelül legyen
        // Ha üres a lista (nincs .recipe-list), akkor a main-be szúrjuk
        listContainer.insertAdjacentHTML('afterbegin', loadingHTML);
        
        // Ha volt "empty-state" üzenet ("Nincsenek receptek"), azt elrejthetjük
        const emptyState = document.querySelector('.empty-state');
        if (emptyState) emptyState.style.display = 'none';
    }
}

// --- 2. Toast UI Setup (Változatlan) ---
function setupToastUI() {
    if (document.getElementById('toast-style')) return;
    const style = document.createElement('style');
    style.id = 'toast-style';
    style.innerHTML = `
        #toast-container { position: fixed; top: 20px; right: 20px; z-index: 9999; display: flex; flex-direction: column; gap: 10px; }
        .toast { min-width: 250px; padding: 16px; border-radius: 12px; background: white; box-shadow: 0 10px 15px -3px rgba(0,0,0,0.1); display: flex; align-items: center; gap: 12px; animation: slideIn 0.3s ease-out forwards; border-left: 6px solid; font-family: 'Inter', sans-serif; font-size: 0.9rem; }
        .toast.success { border-color: #10b981; color: #064e3b; }
        .toast.error { border-color: #ef4444; color: #7f1d1d; }
        .toast.info { border-color: #3b82f6; color: #1e3a8a; }
        .toast-close { cursor: pointer; opacity: 0.5; margin-left: auto; }
        @keyframes slideIn { from { transform: translateX(100%); opacity: 0; } to { transform: translateX(0); opacity: 1; } }
        @keyframes fadeOut { to { transform: translateX(100%); opacity: 0; } }
    `;
    document.head.appendChild(style);
    if (!document.getElementById('toast-container')) {
        const container = document.createElement('div');
        container.id = 'toast-container';
        document.body.appendChild(container);
    }
}

function showToast(message, type = 'info') {
    const container = document.getElementById('toast-container');
    if (!container) return;
    const toast = document.createElement('div');
    toast.className = `toast ${type}`;
    let icon = type === 'success' ? '✅' : (type === 'error' ? '❌' : 'ℹ️');
    toast.innerHTML = `<div>${icon}</div><div>${message}</div><div class="toast-close" onclick="this.parentElement.remove()">×</div>`;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.animation = 'fadeOut 0.3s ease-in forwards';
        toast.addEventListener('animationend', () => toast.remove());
    }, 5000);
}

// --- 3. Polling Logika (Kicsit módosítva) ---
function checkJobStatus() {
    const activeJobId = localStorage.getItem('active_job_id');
    if (!activeJobId) return;

    fetch(`/status?id=${activeJobId}`)
        .then(res => {
            if (!res.ok) { 
                localStorage.removeItem('active_job_id');
                // Ha hiba van (pl 404), vegyük ki a loading kártyát is!
                const loadingCard = document.getElementById('temp-loading-card');
                if (loadingCard) loadingCard.remove();
                return null; 
            }
            return res.json();
        })
        .then(data => {
            if (!data) return;

            if (data.status === 'done') {
                showToast(`✅ Kész! ${data.result.recept_neve} elkészült.`, 'success');
                localStorage.removeItem('active_job_id');
                
                // Ha a receptek oldalon vagyunk
                if (window.location.pathname === '/receptek') {
                    // Opcionális: A loading kártyát átalakíthatnánk az igazivá, 
                    // de egyszerűbb újratölteni az oldalt, hogy a Go renderelje le.
                    setTimeout(() => location.reload(), 1000);
                }
            } else if (data.status === 'error') {
                showToast('❌ Hiba történt a feldolgozásban.', 'error');
                localStorage.removeItem('active_job_id');
                
                // Loading kártya eltávolítása hiba esetén
                const loadingCard = document.getElementById('temp-loading-card');
                if (loadingCard) loadingCard.remove();
            }
        })
        .catch(err => console.log("Polling error:", err));
}

// Indítás: 2 másodpercenként
setInterval(checkJobStatus, 2000);