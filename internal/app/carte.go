package app

import (
	"log"

	"github.com/louis49/purgatorius/internal/cache"
	"github.com/louis49/purgatorius/internal/comptes"
)

// lecteur renvoie la connexion de lecture du cache d'un compte, ouverte a la
// premiere demande puis gardee (le survol du treemap enchaine les requetes).
// Distincte de celle de la synchro : SQLite en WAL laisse lire pendant
// l'ecriture.
func (a *App) lecteur(id string) (*cache.Cache, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.lecteurs[id]; ok {
		return c, nil
	}
	if err := a.verifierCompte(id); err != nil {
		return nil, err
	}
	c, err := a.ouvrirCache(id)
	if err != nil {
		return nil, err
	}
	a.lecteurs[id] = c
	return c, nil
}

// EvPurgatoireMisDeCote : le Purgatoire d'un compte, illisible, vient d'etre
// mis de cote et remplace par un neuf (vide).
const EvPurgatoireMisDeCote = "purgatoire:misdecote"

// AvisMisDeCote : donnees de EvPurgatoireMisDeCote.
type AvisMisDeCote struct {
	Compte  string `json:"compte"`
	Fichier string `json:"fichier"` // nom du fichier garde, dans travail/comptes/<id>/
}

// ouvrirCache ouvre le cache d'un compte. Si son Purgatoire a du etre mis de
// cote, l'avis est retenu (PurgatoireMisDeCote : la mise de cote a souvent
// lieu au demarrage, avant que le front n'ecoute) et emis.
func (a *App) ouvrirCache(id string) (*cache.Cache, error) {
	c, err := cache.Ouvrir(id)
	if err != nil {
		return nil, err
	}
	if c.PurgatoireMisDeCote != "" {
		a.muAvis.Lock()
		a.misDeCote[id] = c.PurgatoireMisDeCote
		a.muAvis.Unlock()
		a.emettre(EvPurgatoireMisDeCote, AvisMisDeCote{Compte: id, Fichier: c.PurgatoireMisDeCote})
	}
	return c, nil
}

// PurgatoireMisDeCote renvoie, une seule fois, le nom du fichier sous lequel
// le Purgatoire illisible du compte a ete mis de cote ("" si rien).
func (a *App) PurgatoireMisDeCote(id string) string {
	a.muAvis.Lock()
	defer a.muAvis.Unlock()
	f := a.misDeCote[id]
	delete(a.misDeCote, id)
	return f
}

// fermerLecteur libere la connexion de lecture d'un compte (suppression).
func (a *App) fermerLecteur(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if c, ok := a.lecteurs[id]; ok {
		c.Fermer()
		delete(a.lecteurs, id)
	}
}

// Carte renvoie un niveau du treemap du compte.
func (a *App) Carte(id string, r cache.RequeteCarte) (cache.Niveau, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Niveau{}, err
	}
	return c.Carte(r)
}

// DetailCarte renvoie le detail d'un bloc (chemin jusqu'au bloc, ou UID > 0
// pour un message).
func (a *App) DetailCarte(id string, q cache.RequeteDetail) (cache.Detail, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Detail{}, err
	}
	return c.DetailCarte(q)
}

// IndexListe renvoie l'ordre de la liste des messages du dernier niveau (UID
// et taille de chacun).
func (a *App) IndexListe(id string, r cache.RequeteListe) (cache.IndexListe, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.IndexListe{}, err
	}
	return c.IndexListe(r)
}

// LignesListe renvoie le contenu des lignes demandees (au plus
// cache.MaxLignes), dans l'ordre demande.
func (a *App) LignesListe(id string, uids []uint32) ([]cache.LigneMessage, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return nil, err
	}
	return c.Lignes(uids)
}

// EnvoyerPurgatoire met des messages (UID) au Purgatoire.
func (a *App) EnvoyerPurgatoire(id string, uids []uint32) (cache.Lot, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Lot{}, err
	}
	return c.EnvoyerPurgatoire(uids)
}

// EnvoyerBlocPurgatoire met tout un bloc de la carte au Purgatoire.
func (a *App) EnvoyerBlocPurgatoire(id string, r cache.RequeteBloc) (cache.Lot, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Lot{}, err
	}
	return c.EnvoyerBlocPurgatoire(r)
}

// RetirerPurgatoire sort des messages du Purgatoire (annulation d'un envoi,
// ou retrait depuis la revue) ; ils reviennent dans la carte.
func (a *App) RetirerPurgatoire(id string, uids []uint32) (cache.Lot, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Lot{}, err
	}
	return c.RetirerPurgatoire(uids)
}

// ViderPurgatoire retire tout le Purgatoire ; rien ne part chez Gmail.
func (a *App) ViderPurgatoire(id string) (cache.Lot, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.Lot{}, err
	}
	return c.ViderPurgatoire()
}

// EtatPurgatoire : contenu du Purgatoire et totaux de la boite, pour les jauges.
func (a *App) EtatPurgatoire(id string) (cache.EtatPurgatoire, error) {
	c, err := a.lecteur(id)
	if err != nil {
		return cache.EtatPurgatoire{}, err
	}
	return c.EtatDuPurgatoire()
}

// verifierCaches ouvre le cache de chaque compte synchronise. Un cache recree
// vide (schema anterieur, fichier illisible) ou sans UIDVALIDITY (jamais
// synchronise : dossier cache/ efface a la main) fait repasser le compte par
// la synchro, au lieu d'une carte vide ou partielle presentee comme complete.
func (a *App) verifierCaches() {
	r, err := comptes.Lire()
	if err != nil {
		return
	}
	for _, cp := range r.Comptes {
		if !cp.SynchroComplete {
			continue
		}
		c, err := a.lecteur(cp.ID)
		if err != nil {
			continue
		}
		raison := "schema anterieur ou fichier illisible"
		if !c.Reinitialise {
			if v, err := c.Meta("uidvalidity"); err != nil || v != "" {
				continue
			}
			raison = "cache absent"
		}
		log.Printf("cache %s recree (%s) : resynchronisation", cp.ID, raison)
		if _, err := comptes.Modifier(func(reg *comptes.Registre) error {
			if x, ok := reg.Compte(cp.ID); ok {
				x.SynchroComplete = false
			}
			return nil
		}); err != nil {
			log.Printf("compte %s non repasse en synchro : %v", cp.ID, err)
		}
	}
}
