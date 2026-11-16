package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/louis49/purgatorius/internal/comptes"
	"github.com/louis49/purgatorius/internal/secrets"
	"github.com/louis49/purgatorius/internal/store"
	"github.com/louis49/purgatorius/internal/synchro"
)

// CompteVue : un compte tel que l'IHM l'affiche (jamais de secret).
type CompteVue struct {
	ID              string `json:"id"`
	Nom             string `json:"nom"`
	Adresse         string `json:"adresse"`
	Couleur         string `json:"couleur"`
	SynchroComplete bool   `json:"synchroComplete"`
	DerniereSynchro string `json:"derniereSynchro"` // RFC 3339, vide si jamais
}

// EtatApp : ce dont l'IHM a besoin au lancement.
type EtatApp struct {
	Comptes     []CompteVue `json:"comptes"`
	Actif       string      `json:"actif"`
	Langue      string      `json:"langue"`
	Version     string      `json:"version"`
	CoffreRepli bool        `json:"coffreRepli"` // Linux sans coffre : secrets en fichier 0600
	Erreur      string      `json:"erreur"`      // registre illisible, par exemple
}

func vue(c comptes.Compte) CompteVue {
	v := CompteVue{ID: c.ID, Nom: c.Nom, Adresse: c.Adresse, Couleur: c.Couleur, SynchroComplete: c.SynchroComplete}
	if c.DerniereSynchro != nil {
		v.DerniereSynchro = c.DerniereSynchro.Format(time.RFC3339)
	}
	return v
}

// Etat renvoie les comptes, le compte actif et la langue.
func (a *App) Etat() EtatApp {
	e := EtatApp{Version: a.version, Comptes: []CompteVue{}, CoffreRepli: secrets.Repli()}
	r, err := comptes.Lire()
	if err != nil {
		e.Erreur = err.Error()
		return e
	}
	for _, c := range r.Comptes {
		e.Comptes = append(e.Comptes, vue(c))
	}
	e.Actif, e.Langue = r.Actif, r.Langue
	if _, ok := r.Compte(e.Actif); !ok && len(r.Comptes) > 0 {
		e.Actif = r.Comptes[0].ID
	}
	return e
}

// ResultatTest : issue d'un test de connexion.
type ResultatTest struct {
	OK       bool   `json:"ok"`
	Messages int    `json:"messages"`
	Code     string `json:"code"`   // voir synchro.CodeErreur
	Detail   string `json:"detail"` // texte du serveur, jamais le mot de passe
}

// TesterConnexion verifie l'adresse et le mot de passe d'application, en
// lecture seule, et compte les messages de « Tous les messages ».
func (a *App) TesterConnexion(adresse, motDePasse string) ResultatTest {
	adresse = comptes.NormaliserAdresse(adresse)
	if !comptes.AdresseValide(adresse) {
		return ResultatTest{Code: "adresse_invalide"}
	}
	ctx, annuler := context.WithTimeout(a.contexte(), 45*time.Second)
	defer annuler()
	n, err := synchro.Tester(ctx, synchro.Options{Adresse: adresse, MotDePasse: motDePasse, Connecter: a.connecterIMAP})
	if err != nil {
		return ResultatTest{Code: synchro.CodeErreur(err), Detail: err.Error()}
	}
	return ResultatTest{OK: true, Messages: n}
}

// ChangerMotDePasse remplace le mot de passe d'application d'un compte (par
// exemple apres sa revocation : Google les revoque quand le mot de passe du
// compte change). Le nouveau est d'abord teste aupres de Gmail, en lecture
// seule ; il n'est enregistre que si le test reussit. Une synchro en cours
// (avec l'ancien) est arretee : la suivante prendra le nouveau.
func (a *App) ChangerMotDePasse(id, motDePasse string) ResultatTest {
	r, err := comptes.Lire()
	if err != nil {
		return ResultatTest{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	c, ok := r.Compte(id)
	if !ok {
		return ResultatTest{Code: synchro.ErreurAutre, Detail: "compte inconnu"}
	}
	mdp := strings.ReplaceAll(motDePasse, " ", "")
	if mdp == "" {
		return ResultatTest{Code: synchro.ErreurAuthentification}
	}
	res := a.TesterConnexion(c.Adresse, mdp)
	if !res.OK {
		return res
	}
	if err := a.arreterSynchro(id, 10*time.Second); err != nil {
		return ResultatTest{Code: synchro.ErreurAutre, Detail: err.Error()}
	}
	if err := secrets.Ecrire(secrets.Cle{ID: id, Adresse: c.Adresse}, mdp); err != nil {
		return ResultatTest{Code: "coffre", Detail: err.Error()}
	}
	return res
}

// NouveauCompte : saisie du formulaire de creation de profil.
type NouveauCompte struct {
	Nom        string `json:"nom"`
	Adresse    string `json:"adresse"`
	Couleur    string `json:"couleur"`
	MotDePasse string `json:"motDePasse"`
}

// AjouterCompte range le mot de passe dans le coffre, enregistre le compte (ou
// met a jour un compte de meme adresse) et en fait le compte actif.
func (a *App) AjouterCompte(n NouveauCompte) (CompteVue, error) {
	adresse := comptes.NormaliserAdresse(n.Adresse)
	if !comptes.AdresseValide(adresse) {
		return CompteVue{}, errors.New("adresse invalide")
	}
	if !comptes.CouleurValide(n.Couleur) {
		n.Couleur = "#60A5FA" // bleu de la palette « Bonbon »
	}
	nom := strings.TrimSpace(n.Nom)
	if nom == "" {
		nom = adresse
	}
	mdp := strings.ReplaceAll(n.MotDePasse, " ", "")
	if mdp == "" {
		return CompteVue{}, errors.New("mot de passe d'application vide")
	}
	id := comptes.ID(adresse)
	if err := secrets.Ecrire(secrets.Cle{ID: id, Adresse: adresse}, mdp); err != nil {
		return CompteVue{}, fmt.Errorf("coffre du systeme : %w", err)
	}
	var cv CompteVue
	_, err := comptes.Modifier(func(r *comptes.Registre) error {
		if c, ok := r.Compte(id); ok {
			c.Nom, c.Couleur = nom, n.Couleur
			cv = vue(*c)
		} else {
			c := comptes.Compte{ID: id, Nom: nom, Adresse: adresse, Couleur: n.Couleur, Mode: comptes.ModeMotDePasseApp, CreeLe: time.Now().UTC()}
			r.Comptes = append(r.Comptes, c)
			cv = vue(c)
		}
		r.Actif = id
		return nil
	})
	return cv, err
}

// ModifierCompte change le nom affiche et la couleur d'un compte. Un nom vide
// redevient l'adresse ; une couleur invalide est refusee.
func (a *App) ModifierCompte(id, nom, couleur string) (CompteVue, error) {
	if !comptes.CouleurValide(couleur) {
		return CompteVue{}, errors.New("couleur invalide")
	}
	nom = strings.TrimSpace(nom)
	var cv CompteVue
	_, err := comptes.Modifier(func(r *comptes.Registre) error {
		c, ok := r.Compte(id)
		if !ok {
			return errors.New("compte inconnu")
		}
		if nom == "" {
			nom = c.Adresse
		}
		c.Nom, c.Couleur = nom, couleur
		cv = vue(*c)
		return nil
	})
	return cv, err
}

// ChoisirCompte fait de id le compte actif.
func (a *App) ChoisirCompte(id string) error {
	_, err := comptes.Modifier(func(r *comptes.Registre) error {
		if _, ok := r.Compte(id); !ok {
			return errors.New("compte inconnu")
		}
		r.Actif = id
		return nil
	})
	return err
}

// DefinirLangue memorise la langue de l'interface ("fr" ou "en").
func (a *App) DefinirLangue(langue string) error {
	if langue != "fr" && langue != "en" {
		return errors.New("langue non prise en charge")
	}
	_, err := comptes.Modifier(func(r *comptes.Registre) error { r.Langue = langue; return nil })
	return err
}

// SupprimerCompte arrete sa synchro (et attend qu'elle ait fini : elle
// pourrait recreer le cache ou, sous Windows, en bloquer l'effacement), efface
// son secret, puis son cache et ses donnees, et le retire du registre. Dans
// cet ordre : un echec ne detruit jamais le Purgatoire (travail de
// l'utilisateur, non re-derivable) avant que le secret ne soit efface, et
// chaque etape tolere ce qui est deja absent, si bien qu'un nouvel essai
// reprend la ou le precedent s'est arrete. Pendant la suppression, rien ne
// peut rouvrir le compte (verifierCompte). Rien n'est modifie dans Gmail.
func (a *App) SupprimerCompte(id string) error {
	a.mu.Lock()
	if a.suppressions[id] {
		a.mu.Unlock()
		return errSuppression
	}
	if a.corbeilles[id] {
		a.mu.Unlock()
		return errCorbeilleEnCours
	}
	a.suppressions[id] = true
	a.mu.Unlock()
	defer func() {
		a.mu.Lock()
		delete(a.suppressions, id)
		a.mu.Unlock()
	}()
	if err := a.arreterSynchro(id, 10*time.Second); err != nil {
		return err
	}
	a.fermerLecteur(id)
	r, err := comptes.Lire()
	if err != nil {
		return err
	}
	c, ok := r.Compte(id)
	if !ok {
		return errors.New("compte inconnu")
	}
	if err := a.supprimerSecret(secrets.Cle{ID: id, Adresse: c.Adresse}); err != nil {
		return fmt.Errorf("coffre du systeme : %w", err)
	}
	for _, d := range []string{"cache/comptes/" + id, "travail/comptes/" + id} {
		if err := store.RemoveDir(d); err != nil {
			return err
		}
	}
	_, err = comptes.Modifier(func(r *comptes.Registre) error {
		reste := r.Comptes[:0]
		for _, x := range r.Comptes {
			if x.ID != id {
				reste = append(reste, x)
			}
		}
		r.Comptes = reste
		if r.Actif == id {
			r.Actif = ""
			if len(reste) > 0 {
				r.Actif = reste[0].ID
			}
		}
		return nil
	})
	return err
}

func (a *App) contexte() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}
