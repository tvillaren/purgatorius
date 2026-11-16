export namespace app {
	
	export class CompteVue {
	    id: string;
	    nom: string;
	    adresse: string;
	    couleur: string;
	    synchroComplete: boolean;
	    derniereSynchro: string;
	
	    static createFrom(source: any = {}) {
	        return new CompteVue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.nom = source["nom"];
	        this.adresse = source["adresse"];
	        this.couleur = source["couleur"];
	        this.synchroComplete = source["synchroComplete"];
	        this.derniereSynchro = source["derniereSynchro"];
	    }
	}
	export class EtatApp {
	    comptes: CompteVue[];
	    actif: string;
	    langue: string;
	    version: string;
	    coffreRepli: boolean;
	    erreur: string;
	
	    static createFrom(source: any = {}) {
	        return new EtatApp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.comptes = this.convertValues(source["comptes"], CompteVue);
	        this.actif = source["actif"];
	        this.langue = source["langue"];
	        this.version = source["version"];
	        this.coffreRepli = source["coffreRepli"];
	        this.erreur = source["erreur"];
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
	export class NouveauCompte {
	    nom: string;
	    adresse: string;
	    couleur: string;
	    motDePasse: string;
	
	    static createFrom(source: any = {}) {
	        return new NouveauCompte(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nom = source["nom"];
	        this.adresse = source["adresse"];
	        this.couleur = source["couleur"];
	        this.motDePasse = source["motDePasse"];
	    }
	}
	export class ResultatCorbeille {
	    deplaces: number;
	    taille: number;
	    absents: number;
	    ambigus: number;
	    code: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new ResultatCorbeille(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deplaces = source["deplaces"];
	        this.taille = source["taille"];
	        this.absents = source["absents"];
	        this.ambigus = source["ambigus"];
	        this.code = source["code"];
	        this.detail = source["detail"];
	    }
	}
	export class ResultatTest {
	    ok: boolean;
	    messages: number;
	    code: string;
	    detail: string;
	
	    static createFrom(source: any = {}) {
	        return new ResultatTest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.messages = source["messages"];
	        this.code = source["code"];
	        this.detail = source["detail"];
	    }
	}

}

export namespace cache {
	
	export class DetailMessage {
	    objet: string;
	    expediteur: string;
	    nom: string;
	    date: number;
	    taille: number;
	    lu: boolean;
	    categorie: string;
	    libelles: string[];
	    gmMsgId: string;
	    boite: string;
	    destinataires: string[];
	
	    static createFrom(source: any = {}) {
	        return new DetailMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.objet = source["objet"];
	        this.expediteur = source["expediteur"];
	        this.nom = source["nom"];
	        this.date = source["date"];
	        this.taille = source["taille"];
	        this.lu = source["lu"];
	        this.categorie = source["categorie"];
	        this.libelles = source["libelles"];
	        this.gmMsgId = source["gmMsgId"];
	        this.boite = source["boite"];
	        this.destinataires = source["destinataires"];
	    }
	}
	export class Repartition {
	    cle: string;
	    libelle: string;
	    nombre: number;
	    taille: number;
	
	    static createFrom(source: any = {}) {
	        return new Repartition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cle = source["cle"];
	        this.libelle = source["libelle"];
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
	    }
	}
	export class Detail {
	    nombre: number;
	    taille: number;
	    premiere: number;
	    derniere: number;
	    nonLus: number;
	    parAnnee: Repartition[];
	    expediteurs: Repartition[];
	    destinataires: Repartition[];
	    pieces: Repartition[];
	    categories: Repartition[];
	    message?: DetailMessage;
	
	    static createFrom(source: any = {}) {
	        return new Detail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
	        this.premiere = source["premiere"];
	        this.derniere = source["derniere"];
	        this.nonLus = source["nonLus"];
	        this.parAnnee = this.convertValues(source["parAnnee"], Repartition);
	        this.expediteurs = this.convertValues(source["expediteurs"], Repartition);
	        this.destinataires = this.convertValues(source["destinataires"], Repartition);
	        this.pieces = this.convertValues(source["pieces"], Repartition);
	        this.categories = this.convertValues(source["categories"], Repartition);
	        this.message = this.convertValues(source["message"], DetailMessage);
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
	
	export class EtatPurgatoire {
	    nombre: number;
	    taille: number;
	    pj: number;
	    nonLus: number;
	    totalNombre: number;
	    totalTaille: number;
	
	    static createFrom(source: any = {}) {
	        return new EtatPurgatoire(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
	        this.pj = source["pj"];
	        this.nonLus = source["nonLus"];
	        this.totalNombre = source["totalNombre"];
	        this.totalTaille = source["totalTaille"];
	    }
	}
	export class Filtre {
	    axe: string;
	    valeur: string;
	    exclure: string[];
	
	    static createFrom(source: any = {}) {
	        return new Filtre(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.axe = source["axe"];
	        this.valeur = source["valeur"];
	        this.exclure = source["exclure"];
	    }
	}
	export class IndexListe {
	    uids: number[];
	    tailles: number[];
	
	    static createFrom(source: any = {}) {
	        return new IndexListe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uids = source["uids"];
	        this.tailles = source["tailles"];
	    }
	}
	export class LigneMessage {
	    uid: number;
	    gmMsgId: string;
	    objet: string;
	    expediteur: string;
	    nom: string;
	    destinataire: string;
	    date: number;
	    taille: number;
	    pj: number;
	    lu: boolean;
	    boite: string;
	    libelles: string[];
	
	    static createFrom(source: any = {}) {
	        return new LigneMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uid = source["uid"];
	        this.gmMsgId = source["gmMsgId"];
	        this.objet = source["objet"];
	        this.expediteur = source["expediteur"];
	        this.nom = source["nom"];
	        this.destinataire = source["destinataire"];
	        this.date = source["date"];
	        this.taille = source["taille"];
	        this.pj = source["pj"];
	        this.lu = source["lu"];
	        this.boite = source["boite"];
	        this.libelles = source["libelles"];
	    }
	}
	export class Lot {
	    uids: number[];
	    nombre: number;
	    taille: number;
	    ecart: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Lot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uids = source["uids"];
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
	        this.ecart = source["ecart"];
	    }
	}
	export class Noeud {
	    cle: string;
	    libelle: string;
	    domaine: string;
	    nombre: number;
	    taille: number;
	    autres: number;
	    exclure: string[];
	
	    static createFrom(source: any = {}) {
	        return new Noeud(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cle = source["cle"];
	        this.libelle = source["libelle"];
	        this.domaine = source["domaine"];
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
	        this.autres = source["autres"];
	        this.exclure = source["exclure"];
	    }
	}
	export class Niveau {
	    axe: string;
	    noeuds: Noeud[];
	    nombre: number;
	    taille: number;
	
	    static createFrom(source: any = {}) {
	        return new Niveau(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.axe = source["axe"];
	        this.noeuds = this.convertValues(source["noeuds"], Noeud);
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
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
	
	
	export class RequeteBloc {
	    chemin: Filtre[];
	    boite: string;
	    lecture: string;
	    nombre: number;
	    taille: number;
	
	    static createFrom(source: any = {}) {
	        return new RequeteBloc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chemin = this.convertValues(source["chemin"], Filtre);
	        this.boite = source["boite"];
	        this.lecture = source["lecture"];
	        this.nombre = source["nombre"];
	        this.taille = source["taille"];
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
	export class RequeteCarte {
	    axes: string[];
	    chemin: Filtre[];
	    metrique: string;
	    boite: string;
	    lecture: string;
	
	    static createFrom(source: any = {}) {
	        return new RequeteCarte(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.axes = source["axes"];
	        this.chemin = this.convertValues(source["chemin"], Filtre);
	        this.metrique = source["metrique"];
	        this.boite = source["boite"];
	        this.lecture = source["lecture"];
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
	export class RequeteDetail {
	    chemin: Filtre[];
	    uid: number;
	    metrique: string;
	    boite: string;
	    lecture: string;
	
	    static createFrom(source: any = {}) {
	        return new RequeteDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chemin = this.convertValues(source["chemin"], Filtre);
	        this.uid = source["uid"];
	        this.metrique = source["metrique"];
	        this.boite = source["boite"];
	        this.lecture = source["lecture"];
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
	export class RequeteListe {
	    chemin: Filtre[];
	    boite: string;
	    lecture: string;
	    tri: string;
	    desc: boolean;
	    purgatoire: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RequeteListe(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chemin = this.convertValues(source["chemin"], Filtre);
	        this.boite = source["boite"];
	        this.lecture = source["lecture"];
	        this.tri = source["tri"];
	        this.desc = source["desc"];
	        this.purgatoire = source["purgatoire"];
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
	export class Resume {
	    messages: number;
	    expediteurs: number;
	    domaines: number;
	    fils: number;
	    taille: number;
	    premiereDate: number;
	    derniereDate: number;
	
	    static createFrom(source: any = {}) {
	        return new Resume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.messages = source["messages"];
	        this.expediteurs = source["expediteurs"];
	        this.domaines = source["domaines"];
	        this.fils = source["fils"];
	        this.taille = source["taille"];
	        this.premiereDate = source["premiereDate"];
	        this.derniereDate = source["derniereDate"];
	    }
	}

}

